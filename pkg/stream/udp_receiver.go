package stream

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// UDPReceiver handles high-throughput UDP data reception with 1024-byte payloads
type UDPReceiver struct {
	config     *StreamConfig
	conn       *net.UDPConn
	stats      *StreamStats
	state      StreamState
	logger     *logrus.Logger
	
	// Link monitoring
	linkStatus     *LinkStatus
	linkMutex      sync.RWMutex
	lastHeartbeat  time.Time
	
	// Continuous mode
	continuousMode bool
	dataSink       string
	outputDir      string
	currentFile    *os.File
	fileMutex      sync.Mutex
	fileCounter    uint64
	ramBuffer      []byte // For RAM data sink
	ramMutex       sync.Mutex
	
	// FEC components
	fecDecoder *FECDecoder
	
	// Packet reassembly
	receivedPackets map[uint64]*Packet
	expectedSeq     uint64
	reassemblyMutex sync.RWMutex
	
	// Performance optimization
	receiveBuffer chan *Packet
	
	// Control
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewUDPReceiver creates a new UDP receiver
func NewUDPReceiver(config *StreamConfig) *UDPReceiver {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize FEC if enabled
	var fecDecoder *FECDecoder
	if config.EnableFEC {
		fecConfig := FECConfig{
			Algorithm:  "xor",
			BlockSize:  100,
			Redundancy: config.FECRedundancy,
			MaxErrors:  10,
		}
		fecDecoder = NewFECDecoder(fecConfig, config.UDPPayloadSize)
	}
	
	return &UDPReceiver{
		config:         config,
		stats:          &StreamStats{StartTime: time.Now()},
		state:          StateIdle,
		logger:         logrus.New(),
		linkStatus:     &LinkStatus{IsConnected: false},
		continuousMode: config.ContinuousMode,
		dataSink:       config.DataSink,
		outputDir:      config.OutputDirectory,
		fecDecoder:     fecDecoder,
		receivedPackets: make(map[uint64]*Packet),
		expectedSeq:    0,
		receiveBuffer:  make(chan *Packet, config.BufferSize),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the UDP receiver and listens for connections
func (r *UDPReceiver) Start() error {
	r.state = StateConnecting
	
	// Resolve local address
	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", r.config.LocalAddr, r.config.Port))
	if err != nil {
		r.state = StateError
		return fmt.Errorf("failed to resolve local address: %w", err)
	}
	
	// Create UDP listener
	r.conn, err = net.ListenUDP("udp", localAddr)
	if err != nil {
		r.state = StateError
		return fmt.Errorf("failed to start UDP listener: %w", err)
	}
	
	// Set UDP options for high performance
	r.conn.SetReadBuffer(r.config.BufferSize)
	r.conn.SetWriteBuffer(r.config.BufferSize)
	
	r.state = StateConnected
	r.linkStatus.IsConnected = true
	r.lastHeartbeat = time.Now()
	r.logger.Infof("UDP receiver listening on %s:%d", r.config.LocalAddr, r.config.Port)
	
	// Initialize data sink
	if err := r.initializeDataSink(); err != nil {
		return fmt.Errorf("failed to initialize data sink: %w", err)
	}
	
	// Start background workers
	r.startBackgroundWorkers()
	
	return nil
}

// initializeDataSink initializes the data sink based on configuration
func (r *UDPReceiver) initializeDataSink() error {
	switch r.dataSink {
	case "disk":
		if r.continuousMode {
			// Create output directory for continuous mode
			if err := os.MkdirAll(r.outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		} else {
			// Create single output file
			if err := r.createOutputFile(); err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
		}
	case "ram":
		// Initialize RAM buffer
		r.ramBuffer = make([]byte, 0, 1024*1024) // Start with 1MB capacity
	case "none":
		// No data sink, just process packets
	default:
		return ErrInvalidDataSink
	}
	
	return nil
}

// createOutputFile creates the output file based on format
func (r *UDPReceiver) createOutputFile() error {
	var err error
	r.currentFile, err = os.Create(r.config.OutputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	
	// Write file header based on format
	switch r.config.FileFormat {
	case "bin":
		// No special header for binary files
	case "pcap":
		// Write pcap file header
		header := make([]byte, 24)
		binary.LittleEndian.PutUint32(header[0:4], 0xa1b2c3d4) // Magic number
		binary.LittleEndian.PutUint16(header[4:6], 2)          // Major version
		binary.LittleEndian.PutUint16(header[6:8], 4)          // Minor version
		binary.LittleEndian.PutUint32(header[8:12], 0)         // Timezone
		binary.LittleEndian.PutUint32(header[12:16], 0)        // Timestamp accuracy
		binary.LittleEndian.PutUint32(header[16:20], 65535)    // Snapshot length
		binary.LittleEndian.PutUint32(header[20:24], 1)        // Link layer type (Ethernet)
		
		_, err = r.currentFile.Write(header)
		if err != nil {
			return fmt.Errorf("failed to write pcap header: %w", err)
		}
	}
	
	return nil
}

// ReceiveFile receives UDP packets with 1024-byte payloads
func (r *UDPReceiver) ReceiveFile() error {
	if r.state != StateConnected {
		return fmt.Errorf("receiver not connected")
	}
	
	r.state = StateTransferring
	r.logger.Info("Starting UDP file reception")
	
	// Start packet receiver
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.receiveUDPPackets()
	}()
	
	// Process received packets
	r.processUDPPackets()
	return nil
}

// receiveUDPPackets continuously receives UDP packets
func (r *UDPReceiver) receiveUDPPackets() {
	buffer := make([]byte, r.config.UDPPayloadSize+PacketHeaderSize)
	
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			// Set read timeout
			r.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			
			n, remoteAddr, err := r.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				r.logger.Errorf("Error reading UDP packet: %v", err)
				return
			}
			
			if n == 0 {
				continue
			}
			
			// Deserialize packet
			packetData := make([]byte, n)
			copy(packetData, buffer[:n])
			
			packet, err := DeserializePacket(packetData)
			if err != nil {
				r.logger.Errorf("Failed to deserialize UDP packet: %v", err)
				r.stats.Errors++
				continue
			}
			
			// Validate packet
			if err := ValidatePacket(packet); err != nil {
				r.logger.Errorf("Invalid UDP packet: %v", err)
				r.stats.Errors++
				continue
			}
			
			// Handle special packets
			if packet.Header.Flags&FlagHeartbeat != 0 {
				// Send ACK for heartbeat
				r.sendUDPACK(packet.Header.SequenceNum, remoteAddr)
				r.lastHeartbeat = time.Now()
				continue
			}
			
			if packet.Header.Flags&FlagEndOfStream != 0 {
				r.logger.Info("Received end-of-stream packet")
				r.state = StateCompleted
				return
			}
			
			if packet.Header.Flags&FlagResync != 0 {
				r.logger.Info("Received resync packet")
				r.sendUDPACK(packet.Header.SequenceNum, remoteAddr)
				continue
			}
			
			// Add to FEC decoder if enabled
			if r.fecDecoder != nil {
				r.fecDecoder.AddPacket(packet)
			}
			
			// Send to processing channel
			select {
			case r.receiveBuffer <- packet:
			default:
				r.logger.Warn("Receive buffer full, dropping packet")
			}
		}
	}
}

// processUDPPackets processes received UDP packets in order
func (r *UDPReceiver) processUDPPackets() {
	for {
		select {
		case packet := <-r.receiveBuffer:
			if err := r.processUDPPacket(packet); err != nil {
				r.logger.Errorf("Failed to process UDP packet %d: %v", packet.Header.SequenceNum, err)
				r.stats.Errors++
			}
		case <-r.ctx.Done():
			return
		}
	}
}

// processUDPPacket processes a single UDP packet
func (r *UDPReceiver) processUDPPacket(packet *Packet) error {
	// Check if this is the expected packet
	if packet.Header.SequenceNum == r.expectedSeq {
		// Write packet payload to data sink
		if err := r.writeToDataSink(packet); err != nil {
			return err
		}
		
		// Update statistics
		r.stats.PacketsReceived++
		r.stats.BytesReceived += uint64(len(packet.Payload))
		r.expectedSeq++
		
		// Check for out-of-order packets that can now be processed
		r.processOutOfOrderUDPPackets()
		
		// Progress logging
		if r.stats.PacketsReceived%1000 == 0 {
			throughput := ConvertToGbps(CalculateThroughput(r.stats.BytesReceived, time.Since(r.stats.StartTime)))
			r.logger.Infof("Received %d packets, Throughput: %.2f Gbps", r.stats.PacketsReceived, throughput)
		}
	} else if packet.Header.SequenceNum > r.expectedSeq {
		// Out-of-order packet, store for later
		r.reassemblyMutex.Lock()
		r.receivedPackets[packet.Header.SequenceNum] = packet
		r.reassemblyMutex.Unlock()
	} else {
		// Duplicate packet, just ignore
	}
	
	return nil
}

// processOutOfOrderUDPPackets processes packets that arrived out of order
func (r *UDPReceiver) processOutOfOrderUDPPackets() {
	r.reassemblyMutex.Lock()
	defer r.reassemblyMutex.Unlock()
	
	for {
		if packet, exists := r.receivedPackets[r.expectedSeq]; exists {
			// Write packet payload to data sink
			if err := r.writeToDataSink(packet); err != nil {
				r.logger.Errorf("Failed to write out-of-order packet %d: %v", r.expectedSeq, err)
			}
			
			// Update statistics
			r.stats.PacketsReceived++
			r.stats.BytesReceived += uint64(len(packet.Payload))
			
			// Remove from map and increment expected sequence
			delete(r.receivedPackets, r.expectedSeq)
			r.expectedSeq++
		} else {
			break
		}
	}
}

// writeToDataSink writes packet payload to the configured data sink
func (r *UDPReceiver) writeToDataSink(packet *Packet) error {
	switch r.dataSink {
	case "disk":
		return r.writeToDisk(packet)
	case "ram":
		return r.writeToRAM(packet)
	case "none":
		// Don't save data, just process
		return nil
	default:
		return ErrInvalidDataSink
	}
}

// writeToDisk writes packet payload to disk
func (r *UDPReceiver) writeToDisk(packet *Packet) error {
	r.fileMutex.Lock()
	defer r.fileMutex.Unlock()
	
	if r.continuousMode {
		// Check if we need to rotate files
		if r.config.EnableFileRotation && r.stats.CurrentFileSize >= r.config.MaxFileSize {
			if err := r.rotateFile(); err != nil {
				return err
			}
		}
	}
	
	switch r.config.FileFormat {
	case "bin":
		// Write raw payload for binary files
		_, err := r.currentFile.Write(packet.Payload)
		if err != nil {
			return err
		}
		r.stats.CurrentFileSize += int64(len(packet.Payload))
		
	case "pcap":
		// Write pcap packet header and payload
		packetLen := len(packet.Payload)
		header := make([]byte, 16)
		
		// Timestamp
		now := time.Now()
		sec := uint32(now.Unix())
		usec := uint32(now.Nanosecond() / 1000)
		binary.LittleEndian.PutUint32(header[0:4], sec)
		binary.LittleEndian.PutUint32(header[4:8], usec)
		
		// Packet length
		binary.LittleEndian.PutUint32(header[8:12], uint32(packetLen))
		binary.LittleEndian.PutUint32(header[12:16], uint32(packetLen))
		
		// Write header
		if _, err := r.currentFile.Write(header); err != nil {
			return err
		}
		
		// Write payload
		if _, err := r.currentFile.Write(packet.Payload); err != nil {
			return err
		}
		
		r.stats.CurrentFileSize += int64(16 + packetLen)
	}
	
	return nil
}

// writeToRAM writes packet payload to RAM buffer
func (r *UDPReceiver) writeToRAM(packet *Packet) error {
	r.ramMutex.Lock()
	defer r.ramMutex.Unlock()
	
	// Append payload to RAM buffer
	r.ramBuffer = append(r.ramBuffer, packet.Payload...)
	
	// Optional: Limit RAM buffer size
	if len(r.ramBuffer) > 100*1024*1024 { // 100MB limit
		r.ramBuffer = r.ramBuffer[len(packet.Payload):] // Remove oldest data
	}
	
	return nil
}

// rotateFile rotates the output file in continuous mode
func (r *UDPReceiver) rotateFile() error {
	if r.currentFile != nil {
		r.currentFile.Close()
	}
	
	r.fileCounter++
	filename := fmt.Sprintf("stream_%d_%s", r.fileCounter, time.Now().Format("20060102_150405"))
	if r.config.FileFormat == "pcap" {
		filename += ".pcap"
	} else {
		filename += ".bin"
	}
	
	filepath := filepath.Join(r.outputDir, filename)
	
	var err error
	r.currentFile, err = os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create rotated file: %w", err)
	}
	
	// Write file header if needed
	if r.config.FileFormat == "pcap" {
		header := make([]byte, 24)
		binary.LittleEndian.PutUint32(header[0:4], 0xa1b2c3d4)
		binary.LittleEndian.PutUint16(header[4:6], 2)
		binary.LittleEndian.PutUint16(header[6:8], 4)
		binary.LittleEndian.PutUint32(header[8:12], 0)
		binary.LittleEndian.PutUint32(header[12:16], 0)
		binary.LittleEndian.PutUint32(header[16:20], 65535)
		binary.LittleEndian.PutUint32(header[20:24], 1)
		
		_, err = r.currentFile.Write(header)
		if err != nil {
			return fmt.Errorf("failed to write pcap header: %w", err)
		}
	}
	
	r.stats.FilesCreated++
	r.stats.CurrentFileSize = 0
	
	r.logger.Infof("Rotated to new file: %s", filename)
	return nil
}

// sendUDPACK sends an acknowledgment for a UDP packet
func (r *UDPReceiver) sendUDPACK(sequenceNum uint64, remoteAddr *net.UDPAddr) {
	ackData := make([]byte, 8)
	binary.BigEndian.PutUint64(ackData, sequenceNum)
	
	// Send ACK (non-blocking)
	go func() {
		r.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		_, err := r.conn.WriteToUDP(ackData, remoteAddr)
		if err != nil {
			r.logger.Debugf("Failed to send UDP ACK for packet %d: %v", sequenceNum, err)
		}
	}()
}

// startBackgroundWorkers starts background goroutines
func (r *UDPReceiver) startBackgroundWorkers() {
	// FEC decoder worker (if enabled)
	if r.fecDecoder != nil {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.fecDecoderWorker()
		}()
	}
	
	// Link monitor (if enabled)
	if r.config.EnableLinkMonitoring {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.monitorLink()
		}()
	}
}

// fecDecoderWorker processes FEC blocks
func (r *UDPReceiver) fecDecoderWorker() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check for complete blocks that can be decoded
			// This is a simplified implementation
		case <-r.ctx.Done():
			return
		}
	}
}

// monitorLink monitors link status
func (r *UDPReceiver) monitorLink() {
	ticker := time.NewTicker(r.config.LinkMonitorInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.checkLinkStatus()
		case <-r.ctx.Done():
			return
		}
	}
}

// checkLinkStatus checks if the link is still active
func (r *UDPReceiver) checkLinkStatus() {
	r.linkMutex.Lock()
	defer r.linkMutex.Unlock()
	
	timeSinceHeartbeat := time.Since(r.lastHeartbeat)
	if timeSinceHeartbeat > r.config.LinkTimeout {
		if r.linkStatus.IsConnected {
			r.logger.Warn("Link interruption detected")
			r.state = StateLinkInterrupted
			r.linkStatus.IsConnected = false
			r.linkStatus.Interruptions++
			r.stats.LinkInterruptions++
		}
	} else {
		if !r.linkStatus.IsConnected {
			r.logger.Info("Link restored")
			r.linkStatus.IsConnected = true
			r.state = StateTransferring
		}
	}
}

// GetStats returns current transfer statistics
func (r *UDPReceiver) GetStats() *StreamStats {
	r.stats.Throughput = CalculateThroughput(r.stats.BytesReceived, time.Since(r.stats.StartTime))
	r.stats.LastLinkStatus = r.linkStatus.IsConnected
	return r.stats
}

// Close closes the UDP receiver and cleans up resources
func (r *UDPReceiver) Close() error {
	r.cancel()
	r.wg.Wait()
	
	if r.currentFile != nil {
		r.currentFile.Close()
	}
	
	if r.conn != nil {
		r.conn.Close()
	}
	
	r.stats.EndTime = time.Now()
	return nil
}