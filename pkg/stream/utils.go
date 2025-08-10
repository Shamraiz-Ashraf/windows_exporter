package stream

import (
	"crypto/rand"
	"hash/crc32"
	"io"
	"time"
)

// CalculateCRC32 calculates CRC32 checksum for the given data
func CalculateCRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// GenerateRandomBytes generates random bytes of specified length
func GenerateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	return bytes, err
}

// GetCurrentTimestamp returns current timestamp in nanoseconds
func GetCurrentTimestamp() uint64 {
	return uint64(time.Now().UnixNano())
}

// TimestampToTime converts timestamp to time.Time
func TimestampToTime(timestamp uint64) time.Time {
	return time.Unix(0, int64(timestamp))
}

// CalculateThroughput calculates throughput in bytes per second
func CalculateThroughput(bytes uint64, duration time.Duration) float64 {
	if duration == 0 {
		return 0
	}
	return float64(bytes) / duration.Seconds()
}

// ConvertToGbps converts bytes per second to Gbps
func ConvertToGbps(bytesPerSecond float64) float64 {
	return bytesPerSecond * 8 / (1024 * 1024 * 1024)
}

// ConvertFromGbps converts Gbps to bytes per second
func ConvertFromGbps(gbps float64) float64 {
	return gbps * 1024 * 1024 * 1024 / 8
}

// ValidatePacket validates a complete packet
func ValidatePacket(packet *Packet) error {
	// Validate header
	if err := packet.Header.Validate(); err != nil {
		return err
	}
	
	// Check payload length matches header
	if uint32(len(packet.Payload)) != packet.Header.Length {
		return ErrInvalidPacketSize
	}
	
	// Verify checksum
	calculatedChecksum := CalculateCRC32(packet.Payload)
	if calculatedChecksum != packet.Header.Checksum {
		return ErrChecksumMismatch
	}
	
	return nil
}

// CreatePacket creates a new packet with the given payload
func CreatePacket(sequenceNum uint64, payload []byte, flags uint16) *Packet {
	checksum := CalculateCRC32(payload)
	
	header := PacketHeader{
		Magic:       DefaultMagic,
		SequenceNum: sequenceNum,
		Length:      uint32(len(payload)),
		Checksum:    checksum,
		Timestamp:   GetCurrentTimestamp(),
		Flags:       flags,
		Reserved:    0,
	}
	
	return &Packet{
		Header:  header,
		Payload: payload,
	}
}

// SerializePacket serializes a complete packet to bytes
func SerializePacket(packet *Packet) []byte {
	headerBytes := packet.Header.Serialize()
	totalSize := len(headerBytes) + len(packet.Payload)
	
	result := make([]byte, totalSize)
	copy(result[0:PacketHeaderSize], headerBytes)
	copy(result[PacketHeaderSize:], packet.Payload)
	
	return result
}

// DeserializePacket deserializes bytes to a complete packet
func DeserializePacket(data []byte) (*Packet, error) {
	if len(data) < PacketHeaderSize {
		return nil, ErrInvalidPacketSize
	}
	
	var header PacketHeader
	if err := header.Deserialize(data[:PacketHeaderSize]); err != nil {
		return nil, err
	}
	
	payload := make([]byte, header.Length)
	if len(data) < PacketHeaderSize+int(header.Length) {
		return nil, ErrInvalidPacketSize
	}
	copy(payload, data[PacketHeaderSize:PacketHeaderSize+int(header.Length)])
	
	packet := &Packet{
		Header:  header,
		Payload: payload,
	}
	
	return packet, nil
}

// ReadFileInChunks reads a file in chunks for efficient streaming
func ReadFileInChunks(reader io.Reader, chunkSize int) chan []byte {
	ch := make(chan []byte, 100) // Buffer channel for performance
	
	go func() {
		defer close(ch)
		
		buffer := make([]byte, chunkSize)
		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buffer[:n])
				ch <- chunk
			}
			if err == io.EOF {
				break
			}
		}
	}()
	
	return ch
}

// CalculateOptimalPacketSize calculates optimal packet size based on target throughput
func CalculateOptimalPacketSize(targetGbps float64, latency time.Duration) int {
	// Convert target throughput to bytes per second
	targetBps := ConvertFromGbps(targetGbps)
	
	// Calculate packet size based on bandwidth-delay product
	// Add some overhead for headers and network efficiency
	latencyMs := latency.Milliseconds()
	if latencyMs == 0 {
		latencyMs = 1 // Default to 1ms if not specified
	}
	
	// Bandwidth-delay product calculation
	bdp := int(targetBps * float64(latencyMs) / 1000)
	
	// Add header overhead and round to reasonable packet size
	packetSize := bdp + PacketHeaderSize
	
	// Ensure packet size is within reasonable bounds
	if packetSize < 1024 {
		packetSize = 1024
	} else if packetSize > MaxPacketSize {
		packetSize = MaxPacketSize
	}
	
	return packetSize
}

// CalculateOptimalBufferSize calculates optimal buffer size for target throughput
func CalculateOptimalBufferSize(targetGbps float64, latency time.Duration) int {
	// Convert target throughput to bytes per second
	targetBps := ConvertFromGbps(targetGbps)
	
	// Calculate buffer size based on bandwidth-delay product
	// Use 2x BDP for safety margin
	latencyMs := latency.Milliseconds()
	if latencyMs == 0 {
		latencyMs = 1 // Default to 1ms if not specified
	}
	
	bufferSize := int(2 * targetBps * float64(latencyMs) / 1000)
	
	// Ensure minimum buffer size
	if bufferSize < DefaultBufferSize {
		bufferSize = DefaultBufferSize
	}
	
	return bufferSize
}