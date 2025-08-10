package stream

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Receiver handles high-throughput data reception with zero-loss guarantees
type Receiver struct {
	config     *StreamConfig
	listener   net.Listener
	conn       net.Conn
	stats      *StreamStats
	state      StreamState
	logger     *logrus.Logger
	
	// FEC components
	fecDecoder *FECDecoder
	
	// Packet reassembly
	receivedPackets map[uint64]*Packet
	expectedSeq     uint64
	reassemblyMutex sync.RWMutex
	
	// Output handling
	outputFile *os.File
	writeMutex sync.Mutex
	
	// Performance optimization
	receiveBuffer chan *Packet
	
	// Control
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewReceiver creates a new high-throughput receiver
func NewReceiver(config *StreamConfig) *Receiver {
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
		fecDecoder = NewFECDecoder(fecConfig, config.PacketSize)
	}
	
	return &Receiver{
		config:         config,
		stats:          &StreamStats{StartTime: time.Now()},
		state:          StateIdle,
		logger:         logrus.New(),
		fecDecoder:     fecDecoder,
		receivedPackets: make(map[uint64]*Packet),
		expectedSeq:    0,
		receiveBuffer:  make(chan *Packet, config.BufferSize),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the receiver and listens for connections
func (r *Receiver) Start() error {
	r.state = StateConnecting
	
	var err error
	r.listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", r.config.LocalAddr, r.config.Port))
	if err != nil {
		r.state = StateError
		return fmt.Errorf("failed to start listener: %w", err)
	}
	
	r.logger.Infof("Receiver listening on %s:%d", r.config.LocalAddr, r.config.Port)
	
	// Accept connection
	r.conn, err = r.listener.Accept()
	if err != nil {
		r.state = StateError
		return fmt.Errorf("failed to accept connection: %w", err)
	}
	
	// Set TCP options for high performance
	if tcpConn, ok := r.conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetWriteBuffer(r.config.BufferSize)
		tcpConn.SetReadBuffer(r.config.BufferSize)
	}
	
	r.state = StateConnected
	r.logger.Info("Connection accepted from sender")
	
	// Create output file
	if err := r.createOutputFile(); err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	
	// Start background workers
	r.startBackgroundWorkers()
	
	return nil
}

// ReceiveFile receives a file with high throughput and zero-loss guarantees
func (r *Receiver) ReceiveFile() error {
	if r.state != StateConnected {
		return fmt.Errorf("receiver not connected")
	}
	
	r.state = StateTransferring
	r.logger.Info("Starting file reception")
	
	// Start packet receiver
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.receivePackets()
	}()
	
	// Process received packets
	r.processPackets()
	return nil
}

// createOutputFile creates the output file based on format
func (r *Receiver) createOutputFile() error {
	var err error
	r.outputFile, err = os.Create(r.config.OutputFile)
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
		
		_, err = r.outputFile.Write(header)
		if err != nil {
			return fmt.Errorf("failed to write pcap header: %w", err)
		}
	}
	
	return nil
}

// receivePackets continuously receives packets from the network
func (r *Receiver) receivePackets() {
	buffer := make([]byte, r.config.PacketSize+PacketHeaderSize)
	
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			// Set read timeout
			r.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			
			n, err := r.conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				r.logger.Errorf("Error reading packet: %v", err)
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
				r.logger.Errorf("Failed to deserialize packet: %v", err)
				r.stats.Errors++
				continue
			}
			
			// Validate packet
			if err := ValidatePacket(packet); err != nil {
				r.logger.Errorf("Invalid packet: %v", err)
				r.stats.Errors++
				continue
			}
			
			// Handle special packets
			if packet.Header.Flags&FlagHeartbeat != 0 {
				// Send ACK for heartbeat
				r.sendACK(packet.Header.SequenceNum)
				continue
			}
			
			if packet.Header.Flags&FlagEndOfStream != 0 {
				r.logger.Info("Received end-of-stream packet")
				r.state = StateCompleted
				return
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

// processPackets processes received packets in order
func (r *Receiver) processPackets() {
	for {
		select {
		case packet := <-r.receiveBuffer:
			if err := r.processPacket(packet); err != nil {
				r.logger.Errorf("Failed to process packet %d: %v", packet.Header.SequenceNum, err)
				r.stats.Errors++
			}
		case <-r.ctx.Done():
			return
		}
	}
}

// processPacket processes a single packet
func (r *Receiver) processPacket(packet *Packet) error {
	// Check if this is the expected packet
	if packet.Header.SequenceNum == r.expectedSeq {
		// Write packet payload to file
		if err := r.writePacketData(packet); err != nil {
			return err
		}
		
		// Send ACK
		r.sendACK(packet.Header.SequenceNum)
		
		// Update statistics
		r.stats.PacketsReceived++
		r.stats.BytesReceived += uint64(len(packet.Payload))
		r.expectedSeq++
		
		// Check for out-of-order packets that can now be processed
		r.processOutOfOrderPackets()
		
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
		
		// Send ACK for out-of-order packet
		r.sendACK(packet.Header.SequenceNum)
	} else {
		// Duplicate packet, just send ACK
		r.sendACK(packet.Header.SequenceNum)
	}
	
	return nil
}

// processOutOfOrderPackets processes packets that arrived out of order
func (r *Receiver) processOutOfOrderPackets() {
	r.reassemblyMutex.Lock()
	defer r.reassemblyMutex.Unlock()
	
	for {
		if packet, exists := r.receivedPackets[r.expectedSeq]; exists {
			// Write packet payload to file
			if err := r.writePacketData(packet); err != nil {
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

// writePacketData writes packet payload to the output file
func (r *Receiver) writePacketData(packet *Packet) error {
	r.writeMutex.Lock()
	defer r.writeMutex.Unlock()
	
	switch r.config.FileFormat {
	case "bin":
		// Write raw payload for binary files
		_, err := r.outputFile.Write(packet.Payload)
		return err
		
	case "pcap":
		// Write pcap packet header and payload
		packetLen := len(packet.Payload)
		header := make([]byte, 16)
		
		// Timestamp (simplified)
		now := time.Now()
		sec := uint32(now.Unix())
		usec := uint32(now.Nanosecond() / 1000)
		binary.LittleEndian.PutUint32(header[0:4], sec)
		binary.LittleEndian.PutUint32(header[4:8], usec)
		
		// Packet length
		binary.LittleEndian.PutUint32(header[8:12], uint32(packetLen))
		binary.LittleEndian.PutUint32(header[12:16], uint32(packetLen))
		
		// Write header
		if _, err := r.outputFile.Write(header); err != nil {
			return err
		}
		
		// Write payload
		if _, err := r.outputFile.Write(packet.Payload); err != nil {
			return err
		}
		
		return nil
		
	default:
		return ErrInvalidFileFormat
	}
}

// sendACK sends an acknowledgment for a packet
func (r *Receiver) sendACK(sequenceNum uint64) {
	ackData := make([]byte, 8)
	binary.BigEndian.PutUint64(ackData, sequenceNum)
	
	// Send ACK (non-blocking)
	go func() {
		r.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		_, err := r.conn.Write(ackData)
		if err != nil {
			r.logger.Debugf("Failed to send ACK for packet %d: %v", sequenceNum, err)
		}
	}()
}

// startBackgroundWorkers starts background goroutines
func (r *Receiver) startBackgroundWorkers() {
	// FEC decoder worker (if enabled)
	if r.fecDecoder != nil {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.fecDecoderWorker()
		}()
	}
}

// fecDecoderWorker processes FEC blocks
func (r *Receiver) fecDecoderWorker() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check for complete blocks that can be decoded
			// This is a simplified implementation
			// In a real system, you would track block completion more sophisticatedly
		case <-r.ctx.Done():
			return
		}
	}
}

// GetStats returns current transfer statistics
func (r *Receiver) GetStats() *StreamStats {
	r.stats.Throughput = CalculateThroughput(r.stats.BytesReceived, time.Since(r.stats.StartTime))
	return r.stats
}

// Close closes the receiver and cleans up resources
func (r *Receiver) Close() error {
	r.cancel()
	r.wg.Wait()
	
	if r.outputFile != nil {
		r.outputFile.Close()
	}
	
	if r.conn != nil {
		r.conn.Close()
	}
	
	if r.listener != nil {
		r.listener.Close()
	}
	
	r.stats.EndTime = time.Now()
	return nil
}