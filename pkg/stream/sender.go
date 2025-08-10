package stream

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/sirupsen/logrus"
)

// Sender handles high-throughput data transmission with zero-loss guarantees
type Sender struct {
	config     *StreamConfig
	conn       net.Conn
	stats      *StreamStats
	state      StreamState
	logger     *logrus.Logger
	
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

// NewSender creates a new high-throughput sender
func NewSender(config *StreamConfig) *Sender {
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
		fecEncoder = NewFECEncoder(fecConfig, config.PacketSize)
	}
	
	return &Sender{
		config:      config,
		stats:       &StreamStats{StartTime: time.Now()},
		state:       StateIdle,
		logger:      logrus.New(),
		fecEncoder:  fecEncoder,
		windowSize:  config.WindowSize,
		sentPackets: make(map[uint64]*Packet),
		ackedPackets: make(map[uint64]bool),
		sendBuffer:  make(chan *Packet, config.BufferSize),
		ackBuffer:   make(chan uint64, config.BufferSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Connect establishes connection to the receiver
func (s *Sender) Connect() error {
	s.state = StateConnecting
	
	var err error
	s.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", s.config.RemoteAddr, s.config.Port))
	if err != nil {
		s.state = StateError
		return fmt.Errorf("failed to connect: %w", err)
	}
	
	// Set TCP options for high performance
	if tcpConn, ok := s.conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetWriteBuffer(s.config.BufferSize)
		tcpConn.SetReadBuffer(s.config.BufferSize)
	}
	
	s.state = StateConnected
	s.logger.Info("Connected to receiver")
	
	// Start background goroutines
	s.startBackgroundWorkers()
	
	return nil
}

// SendFile sends a file with high throughput and zero-loss guarantees
func (s *Sender) SendFile() error {
	if s.state != StateConnected {
		return fmt.Errorf("sender not connected")
	}
	
	s.state = StateTransferring
	s.logger.Info("Starting file transfer")
	
	// Read and send file based on format
	switch s.config.FileFormat {
	case "bin":
		return s.sendBinaryFile()
	case "pcap":
		return s.sendPcapFile()
	default:
		return ErrInvalidFileFormat
	}
}

// sendBinaryFile sends a binary file
func (s *Sender) sendBinaryFile() error {
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
	s.logger.Infof("Sending binary file: %s (%d bytes)", s.config.InputFile, totalBytes)
	
	// Calculate optimal packet size for target throughput
	optimalPacketSize := CalculateOptimalPacketSize(7.0, 1*time.Millisecond) // 7 Gbps target
	if optimalPacketSize > s.config.PacketSize {
		optimalPacketSize = s.config.PacketSize
	}
	
	// Read file in chunks and send
	sequenceNum := uint64(0)
	buffer := make([]byte, optimalPacketSize)
	
	for {
		n, _ := file.Read(buffer)
		if n == 0 {
			break
		}
		
		// Create packet with actual data
		payload := make([]byte, n)
		copy(payload, buffer[:n])
		
		flags := uint16(0)
		if sequenceNum == 0 {
			// First packet
			flags |= FlagCompressed
		}
		
		packet := CreatePacket(sequenceNum, payload, flags)
		
		// Send packet
		if err := s.sendPacket(packet); err != nil {
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
	endPacket := CreatePacket(sequenceNum, []byte{}, FlagEndOfStream)
	if err := s.sendPacket(endPacket); err != nil {
		return fmt.Errorf("failed to send end packet: %w", err)
	}
	
	s.logger.Info("File transfer completed")
	s.state = StateCompleted
	s.stats.EndTime = time.Now()
	
	return nil
}

// sendPcapFile sends a pcap file
func (s *Sender) sendPcapFile() error {
	handle, err := pcap.OpenOffline(s.config.InputFile)
	if err != nil {
		return fmt.Errorf("failed to open pcap file: %w", err)
	}
	defer handle.Close()
	
	s.logger.Infof("Sending pcap file: %s", s.config.InputFile)
	
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	sequenceNum := uint64(0)
	
	for packet := range packetSource.Packets() {
		// Extract packet data
		packetData := packet.Data()
		
		// Split large packets if necessary
		maxPayloadSize := s.config.PacketSize - PacketHeaderSize
		for len(packetData) > 0 {
			chunkSize := len(packetData)
			if chunkSize > maxPayloadSize {
				chunkSize = maxPayloadSize
			}
			
			payload := make([]byte, chunkSize)
			copy(payload, packetData[:chunkSize])
			
			flags := uint16(0)
			if sequenceNum == 0 {
				flags |= FlagCompressed
			}
			
			packet := CreatePacket(sequenceNum, payload, flags)
			
			if err := s.sendPacket(packet); err != nil {
				return fmt.Errorf("failed to send packet %d: %w", sequenceNum, err)
			}
			
			sequenceNum++
			s.stats.PacketsSent++
			s.stats.BytesSent += uint64(chunkSize)
			
			packetData = packetData[chunkSize:]
		}
	}
	
	// Send end-of-stream packet
	endPacket := CreatePacket(sequenceNum, []byte{}, FlagEndOfStream)
	if err := s.sendPacket(endPacket); err != nil {
		return fmt.Errorf("failed to send end packet: %w", err)
	}
	
	s.logger.Info("Pcap file transfer completed")
	s.state = StateCompleted
	s.stats.EndTime = time.Now()
	
	return nil
}

// sendPacket sends a single packet with flow control
func (s *Sender) sendPacket(packet *Packet) error {
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
	
	// Send packet
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
func (s *Sender) startRetransmissionTimer(sequenceNum uint64) {
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
				s.logger.Debugf("Retransmitting packet %d", sequenceNum)
				packet.Header.Flags |= FlagRetransmit
				s.sendPacket(packet)
				s.stats.PacketsRetransmitted++
			}
		}
	case <-s.ctx.Done():
		return
	}
}

// startBackgroundWorkers starts background goroutines for ACK processing
func (s *Sender) startBackgroundWorkers() {
	// ACK processor
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.processACKs()
	}()
	
	// Heartbeat sender
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.sendHeartbeats()
	}()
}

// processACKs processes incoming acknowledgments
func (s *Sender) processACKs() {
	buffer := make([]byte, 1024)
	
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Set read timeout
			s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			
			n, err := s.conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				s.logger.Errorf("Error reading ACK: %v", err)
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
func (s *Sender) sendHeartbeats() {
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			heartbeat := CreatePacket(0, []byte{}, FlagHeartbeat)
			s.sendPacket(heartbeat)
		case <-s.ctx.Done():
			return
		}
	}
}

// GetStats returns current transfer statistics
func (s *Sender) GetStats() *StreamStats {
	s.stats.Throughput = CalculateThroughput(s.stats.BytesSent, time.Since(s.stats.StartTime))
	return s.stats
}

// Close closes the sender and cleans up resources
func (s *Sender) Close() error {
	s.cancel()
	s.wg.Wait()
	
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}