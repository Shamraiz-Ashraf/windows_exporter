package stream

import (
	"encoding/binary"
	"time"
)

// PacketHeader represents the header structure for each data packet
type PacketHeader struct {
	Magic       uint32 // Magic number for packet identification
	SequenceNum uint64 // Sequence number for ordering
	Length      uint32 // Length of payload
	Checksum    uint32 // CRC32 checksum of payload
	Timestamp   uint64 // Timestamp when packet was created
	Flags       uint16 // Various flags (FEC, retransmission, etc.)
	Reserved    uint16 // Reserved for future use
}

// Packet represents a complete packet with header and payload
type Packet struct {
	Header  PacketHeader
	Payload []byte
}

// StreamConfig holds configuration for the stream sender/receiver
type StreamConfig struct {
	// Network configuration
	LocalAddr  string
	RemoteAddr string
	Port       int
	
	// Performance configuration
	BufferSize     int // Size of send/receive buffers
	PacketSize     int // Maximum packet size
	WindowSize     int // Sliding window size for flow control
	
	// FEC configuration
	EnableFEC      bool
	FECRedundancy  float64 // Redundancy factor (e.g., 0.2 = 20% extra packets)
	
	// Timing configuration
	Timeout        time.Duration
	RetryInterval  time.Duration
	HeartbeatInterval time.Duration
	
	// File configuration
	InputFile      string
	OutputFile     string
	FileFormat     string // "bin" or "pcap"
	
	// Logging
	LogLevel       string
	EnableMetrics  bool
}

// StreamStats holds statistics about the stream transfer
type StreamStats struct {
	StartTime      time.Time
	EndTime        time.Time
	BytesSent      uint64
	BytesReceived  uint64
	PacketsSent    uint64
	PacketsReceived uint64
	PacketsLost    uint64
	PacketsRetransmitted uint64
	FECPacketsSent uint64
	FECPacketsUsed uint64
	Throughput     float64 // bytes per second
	Latency        time.Duration
	Errors         uint64
}

// StreamState represents the current state of the stream
type StreamState int

const (
	StateIdle StreamState = iota
	StateConnecting
	StateConnected
	StateTransferring
	StateCompleted
	StateError
)

// FECConfig holds Forward Error Correction configuration
type FECConfig struct {
	Algorithm      string  // "reed-solomon", "ldpc", etc.
	BlockSize      int     // Size of FEC block
	Redundancy     float64 // Redundancy factor
	MaxErrors      int     // Maximum correctable errors per block
}

// Constants
const (
	PacketHeaderSize = 32 // Size of PacketHeader in bytes
	DefaultMagic     = 0xDEADBEEF
	MaxPacketSize    = 65536
	DefaultBufferSize = 1024 * 1024 // 1MB
	DefaultWindowSize = 1000
)

// Packet flags
const (
	FlagFEC          = 0x0001
	FlagRetransmit   = 0x0002
	FlagHeartbeat    = 0x0004
	FlagEndOfStream  = 0x0008
	FlagCompressed   = 0x0010
)

// SerializeHeader serializes the packet header to bytes
func (h *PacketHeader) Serialize() []byte {
	buf := make([]byte, PacketHeaderSize)
	binary.BigEndian.PutUint32(buf[0:4], h.Magic)
	binary.BigEndian.PutUint64(buf[4:12], h.SequenceNum)
	binary.BigEndian.PutUint32(buf[12:16], h.Length)
	binary.BigEndian.PutUint32(buf[16:20], h.Checksum)
	binary.BigEndian.PutUint64(buf[20:28], h.Timestamp)
	binary.BigEndian.PutUint16(buf[28:30], h.Flags)
	binary.BigEndian.PutUint16(buf[30:32], h.Reserved)
	return buf
}

// DeserializeHeader deserializes bytes to packet header
func (h *PacketHeader) Deserialize(data []byte) error {
	if len(data) < PacketHeaderSize {
		return ErrInvalidPacketSize
	}
	
	h.Magic = binary.BigEndian.Uint32(data[0:4])
	h.SequenceNum = binary.BigEndian.Uint64(data[4:12])
	h.Length = binary.BigEndian.Uint32(data[12:16])
	h.Checksum = binary.BigEndian.Uint32(data[16:20])
	h.Timestamp = binary.BigEndian.Uint64(data[20:28])
	h.Flags = binary.BigEndian.Uint16(data[28:30])
	h.Reserved = binary.BigEndian.Uint16(data[30:32])
	
	return nil
}

// ValidateHeader validates the packet header
func (h *PacketHeader) Validate() error {
	if h.Magic != DefaultMagic {
		return ErrInvalidMagic
	}
	if h.Length > MaxPacketSize {
		return ErrPacketTooLarge
	}
	return nil
}