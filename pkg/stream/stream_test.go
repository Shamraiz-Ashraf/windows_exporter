package stream

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

func TestPacketCreation(t *testing.T) {
	payload := []byte("test payload")
	sequenceNum := uint64(12345)
	flags := uint16(FlagCompressed)
	
	packet := CreatePacket(sequenceNum, payload, flags)
	
	if packet.Header.SequenceNum != sequenceNum {
		t.Errorf("Expected sequence number %d, got %d", sequenceNum, packet.Header.SequenceNum)
	}
	
	if packet.Header.Length != uint32(len(payload)) {
		t.Errorf("Expected length %d, got %d", len(payload), packet.Header.Length)
	}
	
	if packet.Header.Flags != flags {
		t.Errorf("Expected flags %d, got %d", flags, packet.Header.Flags)
	}
	
	if !bytes.Equal(packet.Payload, payload) {
		t.Error("Payload mismatch")
	}
}

func TestPacketSerialization(t *testing.T) {
	payload := []byte("test payload")
	packet := CreatePacket(12345, payload, FlagCompressed)
	
	serialized := SerializePacket(packet)
	
	if len(serialized) != PacketHeaderSize+len(payload) {
		t.Errorf("Expected serialized size %d, got %d", PacketHeaderSize+len(payload), len(serialized))
	}
	
	// Verify header magic
	magic := uint32(0)
	for i := 0; i < 4; i++ {
		magic = (magic << 8) | uint32(serialized[i])
	}
	if magic != DefaultMagic {
		t.Errorf("Expected magic %x, got %x", DefaultMagic, magic)
	}
}

func TestPacketDeserialization(t *testing.T) {
	payload := []byte("test payload")
	originalPacket := CreatePacket(12345, payload, FlagCompressed)
	
	serialized := SerializePacket(originalPacket)
	deserialized, err := DeserializePacket(serialized)
	
	if err != nil {
		t.Fatalf("Failed to deserialize packet: %v", err)
	}
	
	if deserialized.Header.SequenceNum != originalPacket.Header.SequenceNum {
		t.Errorf("Sequence number mismatch: expected %d, got %d", 
			originalPacket.Header.SequenceNum, deserialized.Header.SequenceNum)
	}
	
	if deserialized.Header.Length != originalPacket.Header.Length {
		t.Errorf("Length mismatch: expected %d, got %d", 
			originalPacket.Header.Length, deserialized.Header.Length)
	}
	
	if !bytes.Equal(deserialized.Payload, originalPacket.Payload) {
		t.Error("Payload mismatch after deserialization")
	}
}

func TestPacketValidation(t *testing.T) {
	payload := []byte("test payload")
	packet := CreatePacket(12345, payload, 0)
	
	// Valid packet should pass validation
	if err := ValidatePacket(packet); err != nil {
		t.Errorf("Valid packet failed validation: %v", err)
	}
	
	// Test invalid magic
	packet.Header.Magic = 0x12345678
	if err := ValidatePacket(packet); err != ErrInvalidMagic {
		t.Errorf("Expected ErrInvalidMagic, got %v", err)
	}
	
	// Reset magic
	packet.Header.Magic = DefaultMagic
	
	// Test invalid length
	packet.Header.Length = MaxPacketSize + 1
	if err := ValidatePacket(packet); err != ErrPacketTooLarge {
		t.Errorf("Expected ErrPacketTooLarge, got %v", err)
	}
}

func TestCRC32Calculation(t *testing.T) {
	data := []byte("test data for CRC32")
	checksum := CalculateCRC32(data)
	
	// CRC32 should be consistent
	checksum2 := CalculateCRC32(data)
	if checksum != checksum2 {
		t.Error("CRC32 calculation is not consistent")
	}
	
	// Different data should have different checksums
	data2 := []byte("different test data")
	checksum3 := CalculateCRC32(data2)
	if checksum == checksum3 {
		t.Error("Different data should have different CRC32 checksums")
	}
}

func TestThroughputCalculation(t *testing.T) {
	bytes := uint64(1024 * 1024 * 1024) // 1GB
	duration := 1 * time.Second
	
	throughput := CalculateThroughput(bytes, duration)
	expected := float64(bytes) / duration.Seconds()
	
	if throughput != expected {
		t.Errorf("Expected throughput %f, got %f", expected, throughput)
	}
	
	// Test zero duration
	throughput = CalculateThroughput(bytes, 0)
	if throughput != 0 {
		t.Errorf("Expected throughput 0 for zero duration, got %f", throughput)
	}
}

func TestThroughputConversion(t *testing.T) {
	// Test Gbps to bytes per second
	gbps := 8.0
	bytesPerSecond := ConvertFromGbps(gbps)
	expectedBytesPerSecond := gbps * 1024 * 1024 * 1024 / 8
	
	if bytesPerSecond != expectedBytesPerSecond {
		t.Errorf("Expected %f bytes per second, got %f", expectedBytesPerSecond, bytesPerSecond)
	}
	
	// Test bytes per second to Gbps
	convertedGbps := ConvertToGbps(bytesPerSecond)
	if convertedGbps != gbps {
		t.Errorf("Expected %f Gbps, got %f", gbps, convertedGbps)
	}
}

func TestOptimalPacketSizeCalculation(t *testing.T) {
	targetGbps := 7.0
	latency := 1 * time.Millisecond
	
	packetSize := CalculateOptimalPacketSize(targetGbps, latency)
	
	// Packet size should be reasonable
	if packetSize < 1024 {
		t.Errorf("Packet size too small: %d", packetSize)
	}
	
	if packetSize > MaxPacketSize {
		t.Errorf("Packet size too large: %d", packetSize)
	}
	
	// Should include header size
	if packetSize <= PacketHeaderSize {
		t.Errorf("Packet size should be larger than header size: %d", packetSize)
	}
}

func TestOptimalBufferSizeCalculation(t *testing.T) {
	targetGbps := 7.0
	latency := 1 * time.Millisecond
	
	bufferSize := CalculateOptimalBufferSize(targetGbps, latency)
	
	// Buffer size should be reasonable
	if bufferSize < DefaultBufferSize {
		t.Errorf("Buffer size too small: %d", bufferSize)
	}
	
	// Should be at least 2x bandwidth-delay product
	expectedMin := int(2 * ConvertFromGbps(targetGbps) * float64(latency.Milliseconds()) / 1000)
	if bufferSize < expectedMin {
		t.Errorf("Buffer size %d should be at least %d", bufferSize, expectedMin)
	}
}

func TestFECEncoding(t *testing.T) {
	config := FECConfig{
		Algorithm:  "xor",
		BlockSize:  10,
		Redundancy: 0.2,
		MaxErrors:  2,
	}
	
	encoder := NewFECEncoder(config, 1024)
	
	// Create test packets
	packets := make([]*Packet, 10)
	for i := 0; i < 10; i++ {
		payload := make([]byte, 1024)
		rand.Read(payload)
		packets[i] = CreatePacket(uint64(i), payload, 0)
	}
	
	block, err := encoder.EncodeBlock(packets)
	if err != nil {
		t.Fatalf("Failed to encode FEC block: %v", err)
	}
	
	if block.BlockID != 0 {
		t.Errorf("Expected block ID 0, got %d", block.BlockID)
	}
	
	if len(block.DataPackets) != 10 {
		t.Errorf("Expected 10 data packets, got %d", len(block.DataPackets))
	}
	
	// Should have 2 FEC packets (20% of 10)
	if len(block.FECPackets) != 2 {
		t.Errorf("Expected 2 FEC packets, got %d", len(block.FECPackets))
	}
}

func TestFECDecoding(t *testing.T) {
	config := FECConfig{
		Algorithm:  "xor",
		BlockSize:  10,
		Redundancy: 0.2,
		MaxErrors:  2,
	}
	
	decoder := NewFECDecoder(config, 1024)
	
	// Add packets to decoder
	for i := 0; i < 10; i++ {
		payload := make([]byte, 1024)
		rand.Read(payload)
		packet := CreatePacket(uint64(i), payload, 0)
		decoder.AddPacket(packet)
	}
	
	// Try to decode block
	packets, err := decoder.DecodeBlock(0)
	if err != nil {
		t.Fatalf("Failed to decode FEC block: %v", err)
	}
	
	if len(packets) != 10 {
		t.Errorf("Expected 10 packets, got %d", len(packets))
	}
}

func TestTimestampOperations(t *testing.T) {
	timestamp := GetCurrentTimestamp()
	
	// Timestamp should be recent
	now := time.Now().UnixNano()
	if timestamp < uint64(now-1000000000) { // Within 1 second
		t.Errorf("Timestamp too old: %d", timestamp)
	}
	
	// Convert back to time
	timeFromTimestamp := TimestampToTime(timestamp)
	timeDiff := time.Since(timeFromTimestamp)
	
	if timeDiff > time.Second {
		t.Errorf("Time conversion error: %v", timeDiff)
	}
}

func TestRandomDataGeneration(t *testing.T) {
	length := 1024
	data, err := GenerateRandomBytes(length)
	
	if err != nil {
		t.Fatalf("Failed to generate random bytes: %v", err)
	}
	
	if len(data) != length {
		t.Errorf("Expected length %d, got %d", length, len(data))
	}
	
	// Generate another set and verify they're different
	data2, err := GenerateRandomBytes(length)
	if err != nil {
		t.Fatalf("Failed to generate second random bytes: %v", err)
	}
	
	if bytes.Equal(data, data2) {
		t.Error("Random data should be different")
	}
}

func TestStreamConfigDefaults(t *testing.T) {
	config := &StreamConfig{}
	
	// Test default values
	if config.BufferSize != 0 {
		t.Errorf("Expected default buffer size 0, got %d", config.BufferSize)
	}
	
	if config.PacketSize != 0 {
		t.Errorf("Expected default packet size 0, got %d", config.PacketSize)
	}
	
	if config.WindowSize != 0 {
		t.Errorf("Expected default window size 0, got %d", config.WindowSize)
	}
	
	if config.EnableFEC {
		t.Error("FEC should be disabled by default")
	}
}

func TestStreamStats(t *testing.T) {
	stats := &StreamStats{
		StartTime: time.Now(),
		BytesSent: 1024 * 1024 * 100, // 100MB
		PacketsSent: 1000,
	}
	
	stats.EndTime = stats.StartTime.Add(10 * time.Second)
	
	// Calculate throughput
	throughput := CalculateThroughput(stats.BytesSent, stats.EndTime.Sub(stats.StartTime))
	expectedThroughput := float64(stats.BytesSent) / 10.0
	
	if throughput != expectedThroughput {
		t.Errorf("Expected throughput %f, got %f", expectedThroughput, throughput)
	}
	
	// Convert to Gbps
	throughputGbps := ConvertToGbps(throughput)
	expectedGbps := expectedThroughput * 8 / (1024 * 1024 * 1024)
	
	if throughputGbps != expectedGbps {
		t.Errorf("Expected Gbps %f, got %f", expectedGbps, throughputGbps)
	}
}