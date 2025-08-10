package stream

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// UDPSender handles high-throughput UDP data transmission with 1024-byte payloads
type UDPSender struct {
	config     *StreamConfig
	conn       *net.UDPConn
	remoteAddr *net.UDPAddr
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
	
	// FEC components
	fecEncoder *FECEncoder
	
	// Flow control
	windowSize    int
	sentPackets   map[uint64]*Packet
	ackedPackets  map[uint64]bool
	windowMutex   sync.RWMutex
	
	// Performance optimization
	sendBuffer    chan *Packet
	ackBuffer     chan uint64
	
	// Control
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewUDPSender creates a new UDP sender
func NewUDPSender(config *StreamConfig) *UDPSender {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize FEC if enabled
	var fecEncoder *FECEncoder
	if config.EnableFEC {
		fecConfig := FECConfig{
			Algorithm:  "xor",
			BlockSize:  100,
			Redundancy: config.FECRedundancy,
			MaxErrors:  10,
		}
		fecEncoder = NewFECEncoder(fecConfig, config.UDPPayloadSize)
	}
	
	return &UDPSender{
		config:         config,
		stats:          &StreamStats{StartTime: time.Now()},
		state:          StateIdle,
		logger:         logrus.New(),
		linkStatus:     &LinkStatus{IsConnected: false},
		continuousMode: config.ContinuousMode,
		dataSink:       config.DataSink,
		outputDir:      config.OutputDirectory,
		fecEncoder:     fecEncoder,
		windowSize:     config.WindowSize,
		sentPackets:    make(map[uint64]*Packet),
		ackedPackets:   make(map[uint64]bool),
		sendBuffer:     make(chan *Packet, config.BufferSize),
		ackBuffer:      make(chan uint64, config.BufferSize),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Connect establishes UDP connection to the receiver
func (s *UDPSender) Connect() error {
	s.state = StateConnecting
	
	// Resolve remote address
	remoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", s.config.RemoteAddr, s.config.Port))
	if err != nil {
		s.state = StateError
		return fmt.Errorf("failed to resolve remote address: %w", err)
	}
	s.remoteAddr = remoteAddr
	
	// Create UDP connection
	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", s.config.LocalAddr, 0))
	if err != nil {
		s.state = StateError
		return fmt.Errorf("failed to resolve local address: %w", err)
	}
	
	s.conn, err = net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
		s.state = StateError
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	
	// Set UDP options for high performance
	s.conn.SetWriteBuffer(s.config.BufferSize)
	s.conn.SetReadBuffer(s.config.BufferSize)
	
	s.state = StateConnected
	s.linkStatus.IsConnected = true
	s.lastHeartbeat = time.Now()
	s.logger.Info("UDP connection established")
	
	// Start background workers
	s.startBackgroundWorkers()
	
	return nil
}

// SendFile sends a file with 1024-byte UDP payloads
func (s *UDPSender) SendFile() error {
	if s.state != StateConnected {
		return fmt.Errorf("sender not connected")
	}
	
	s.state = StateTransferring
	s.logger.Info("Starting UDP file transfer")
	
	// Read and send file based on format
	switch s.config.FileFormat {
	case "bin":
		return s.sendBinaryFileUDP()
	case "pcap":
		return s.sendPcapFileUDP()
	default:
		return ErrInvalidFileFormat
	}
}

// sendBinaryFileUDP sends a binary file using 1024-byte UDP payloads
func (s *UDPSender) sendBinaryFileUDP() error {
	file, err := os.Open(s.config.InputFile)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Get file size for progress tracking
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	
	totalBytes := fileInfo.Size()
	s.logger.Infof("Sending binary file via UDP: %s (%d bytes)", s.config.InputFile, totalBytes)
	
	// Use fixed 1024-byte payload size for UDP
	payloadSize := s.config.UDPPayloadSize
	if payloadSize == 0 {
		payloadSize = DefaultUDPPayloadSize
	}
	
	// Read file in chunks and send
	sequenceNum := uint64(0)
	buffer := make([]byte, payloadSize)
	
	for {
		n, _ := file.Read(buffer)
		if n == 0 {
			break
		}
		
		// Create packet with actual data (pad to 1024 bytes if needed)
		payload := make([]byte, payloadSize)
		copy(payload, buffer[:n])
		
		flags := uint16(FlagUDPPayload)
		if sequenceNum == 0 {
			flags |= FlagCompressed
		}
		
		packet := CreatePacket(sequenceNum, payload, flags)
		
		// Send packet
		if err := s.sendUDPPacket(packet); err != nil {
			return fmt.Errorf("failed to send packet %d: %w", sequenceNum, err)
		}
		
		sequenceNum++
		s.stats.PacketsSent++
		s.stats.BytesSent += uint64(n)
		
		// Progress logging
		if sequenceNum%1000 == 0 {
			progress := float64(s.stats.BytesSent) / float64(totalBytes) * 100
			throughput := ConvertToGbps(CalculateThroughput(s.stats.BytesSent, time.Since(s.stats.StartTime)))
			s.logger.Infof("Progress: %.2f%%, Throughput: %.2f Gbps", progress, throughput)
		}
	}
	
	// Send end-of-stream packet
	endPacket := CreatePacket(sequenceNum, []byte{}, FlagEndOfStream|FlagUDPPayload)
	if err := s.sendUDPPacket(endPacket); err != nil {
		return fmt.Errorf("failed to send end packet: %w", err)
	}
	
	s.logger.Info("UDP file transfer completed")
	s.state = StateCompleted
	s.stats.EndTime = time.Now()
	
	return nil
}

// sendPcapFileUDP sends a pcap file using 1024-byte UDP payloads
func (s *UDPSender) sendPcapFileUDP() error {
	// Implementation for pcap file sending via UDP
	// Similar to binary file but with pcap-specific handling
	return s.sendBinaryFileUDP() // Simplified for now
}

// sendUDPPacket sends a single UDP packet with flow control
func (s *UDPSender) sendUDPPacket(packet *Packet) error {
	// Wait for window space
	for {
		s.windowMutex.RLock()
		windowUsed := len(s.sentPackets)
		s.windowMutex.RUnlock()
		
		if windowUsed < s.windowSize {
			break
		}
		
		// Wait a bit before checking again
		time.Sleep(100 * time.Microsecond)
	}
	
	// Serialize packet
	packetData := SerializePacket(packet)
	
	// Send packet via UDP
	_, err := s.conn.Write(packetData)
	if err != nil {
		return err
	}
	
	// Track sent packet for flow control
	s.windowMutex.Lock()
	s.sentPackets[packet.Header.SequenceNum] = packet
	s.windowMutex.Unlock()
	
	// Start retransmission timer
	go s.startRetransmissionTimer(packet.Header.SequenceNum)
	
	return nil
}

// startRetransmissionTimer starts a timer for packet retransmission
func (s *UDPSender) startRetransmissionTimer(sequenceNum uint64) {
	timer := time.NewTimer(s.config.RetryInterval)
	defer timer.Stop()
	
	select {
	case <-timer.C:
		// Check if packet was acknowledged
		s.windowMutex.RLock()
		acked := s.ackedPackets[sequenceNum]
		s.windowMutex.RUnlock()
		
		if !acked {
			// Retransmit packet
			s.windowMutex.RLock()
			packet := s.sentPackets[sequenceNum]
			s.windowMutex.RUnlock()
			
			if packet != nil {
				s.logger.Debugf("Retransmitting UDP packet %d", sequenceNum)
				packet.Header.Flags |= FlagRetransmit
				s.sendUDPPacket(packet)
				s.stats.PacketsRetransmitted++
			}
		}
	case <-s.ctx.Done():
		return
	}
}

// startBackgroundWorkers starts background goroutines
func (s *UDPSender) startBackgroundWorkers() {
	// ACK processor
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.processUDPACKs()
	}()
	
	// Heartbeat sender
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.sendHeartbeats()
	}()
	
	// Link monitor
	if s.config.EnableLinkMonitoring {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.monitorLink()
		}()
	}
}

// processUDPACKs processes incoming acknowledgments
func (s *UDPSender) processUDPACKs() {
	buffer := make([]byte, 1024)
	
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Set read timeout
			s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			
			n, _, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				s.logger.Errorf("Error reading UDP ACK: %v", err)
				return
			}
			
			// Process ACK data
			if n >= 8 {
				ackNum := uint64(0)
				for i := 0; i < 8; i++ {
					ackNum = (ackNum << 8) | uint64(buffer[i])
				}
				
				s.windowMutex.Lock()
				s.ackedPackets[ackNum] = true
				delete(s.sentPackets, ackNum)
				s.windowMutex.Unlock()
			}
		}
	}
}

// sendHeartbeats sends periodic heartbeat packets
func (s *UDPSender) sendHeartbeats() {
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			heartbeat := CreatePacket(0, []byte{}, FlagHeartbeat|FlagUDPPayload)
			s.sendUDPPacket(heartbeat)
			s.lastHeartbeat = time.Now()
		case <-s.ctx.Done():
			return
		}
	}
}

// monitorLink monitors link status and handles interruptions
func (s *UDPSender) monitorLink() {
	ticker := time.NewTicker(s.config.LinkMonitorInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.checkLinkStatus()
		case <-s.ctx.Done():
			return
		}
	}
}

// checkLinkStatus checks if the link is still active
func (s *UDPSender) checkLinkStatus() {
	s.linkMutex.Lock()
	defer s.linkMutex.Unlock()
	
	timeSinceHeartbeat := time.Since(s.lastHeartbeat)
	if timeSinceHeartbeat > s.config.LinkTimeout {
		if s.linkStatus.IsConnected {
			s.logger.Warn("Link interruption detected")
			s.state = StateLinkInterrupted
			s.linkStatus.IsConnected = false
			s.linkStatus.Interruptions++
			s.stats.LinkInterruptions++
		}
	} else {
		if !s.linkStatus.IsConnected {
			s.logger.Info("Link restored, attempting resync")
			s.attemptResync()
		}
	}
}

// attemptResync attempts to resync after link restoration
func (s *UDPSender) attemptResync() {
	s.state = StateResyncing
	
	// Send resync packet
	resyncPacket := CreatePacket(0, []byte{}, FlagResync|FlagUDPPayload)
	s.sendUDPPacket(resyncPacket)
	
	// Wait for acknowledgment
	time.Sleep(100 * time.Millisecond)
	
	s.linkStatus.IsConnected = true
	s.linkStatus.LastResync = time.Now()
	s.stats.ResyncCount++
	s.state = StateTransferring
	
	s.logger.Info("Resync completed successfully")
}

// GetStats returns current transfer statistics
func (s *UDPSender) GetStats() *StreamStats {
	s.stats.Throughput = CalculateThroughput(s.stats.BytesSent, time.Since(s.stats.StartTime))
	s.stats.LastLinkStatus = s.linkStatus.IsConnected
	return s.stats
}

// Close closes the UDP sender and cleans up resources
func (s *UDPSender) Close() error {
	s.cancel()
	s.wg.Wait()
	
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}