package stream

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

// BenchmarkPacketCreation benchmarks packet creation performance
func BenchmarkPacketCreation(b *testing.B) {
	payload := make([]byte, 8192)
	rand.Read(payload)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CreatePacket(uint64(i), payload, 0)
	}
}

// BenchmarkPacketSerialization benchmarks packet serialization
func BenchmarkPacketSerialization(b *testing.B) {
	payload := make([]byte, 8192)
	rand.Read(payload)
	packet := CreatePacket(0, payload, 0)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SerializePacket(packet)
	}
}

// BenchmarkPacketDeserialization benchmarks packet deserialization
func BenchmarkPacketDeserialization(b *testing.B) {
	payload := make([]byte, 8192)
	rand.Read(payload)
	packet := CreatePacket(0, payload, 0)
	packetData := SerializePacket(packet)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DeserializePacket(packetData)
	}
}

// BenchmarkCRC32Calculation benchmarks CRC32 calculation
func BenchmarkCRC32Calculation(b *testing.B) {
	data := make([]byte, 8192)
	rand.Read(data)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateCRC32(data)
	}
}

// BenchmarkFECEncoding benchmarks FEC encoding performance
func BenchmarkFECEncoding(b *testing.B) {
	config := FECConfig{
		Algorithm:  "xor",
		BlockSize:  100,
		Redundancy: 0.2,
		MaxErrors:  10,
	}
	
	encoder := NewFECEncoder(config, 8192)
	
	// Create test packets
	packets := make([]*Packet, 100)
	for i := 0; i < 100; i++ {
		payload := make([]byte, 8192)
		rand.Read(payload)
		packets[i] = CreatePacket(uint64(i), payload, 0)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.EncodeBlock(packets)
	}
}

// BenchmarkFECDecoding benchmarks FEC decoding performance
func BenchmarkFECDecoding(b *testing.B) {
	config := FECConfig{
		Algorithm:  "xor",
		BlockSize:  100,
		Redundancy: 0.2,
		MaxErrors:  10,
	}
	
	decoder := NewFECDecoder(config, 8192)
	
	// Create test packets
	for i := 0; i < 100; i++ {
		payload := make([]byte, 8192)
		rand.Read(payload)
		packet := CreatePacket(uint64(i), payload, 0)
		decoder.AddPacket(packet)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder.DecodeBlock(0)
	}
}

// BenchmarkThroughputCalculation benchmarks throughput calculation
func BenchmarkThroughputCalculation(b *testing.B) {
	bytes := uint64(1024 * 1024 * 1024) // 1GB
	duration := 1 * time.Second
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateThroughput(bytes, duration)
	}
}

// BenchmarkOptimalPacketSizeCalculation benchmarks optimal packet size calculation
func BenchmarkOptimalPacketSizeCalculation(b *testing.B) {
	targetGbps := 7.0
	latency := 1 * time.Millisecond
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateOptimalPacketSize(targetGbps, latency)
	}
}

// BenchmarkOptimalBufferSizeCalculation benchmarks optimal buffer size calculation
func BenchmarkOptimalBufferSizeCalculation(b *testing.B) {
	targetGbps := 7.0
	latency := 1 * time.Millisecond
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateOptimalBufferSize(targetGbps, latency)
	}
}

// BenchmarkPacketValidation benchmarks packet validation
func BenchmarkPacketValidation(b *testing.B) {
	payload := make([]byte, 8192)
	rand.Read(payload)
	packet := CreatePacket(0, payload, 0)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidatePacket(packet)
	}
}

// BenchmarkFileChunkReading benchmarks file chunk reading simulation
func BenchmarkFileChunkReading(b *testing.B) {
	// Simulate file reading with chunks
	chunkSize := 8192
	totalSize := 1024 * 1024 * 100 // 100MB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytesRead := 0
		for bytesRead < totalSize {
			chunk := make([]byte, chunkSize)
			if bytesRead+chunkSize > totalSize {
				chunk = chunk[:totalSize-bytesRead]
			}
			bytesRead += len(chunk)
		}
	}
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	packetSize := 8192
	numPackets := 1000
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		packets := make([]*Packet, numPackets)
		for j := 0; j < numPackets; j++ {
			payload := make([]byte, packetSize)
			packets[j] = CreatePacket(uint64(j), payload, 0)
		}
	}
}

// BenchmarkConcurrentPacketProcessing benchmarks concurrent packet processing
func BenchmarkConcurrentPacketProcessing(b *testing.B) {
	numGoroutines := 4
	packetsPerGoroutine := 1000
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		done := make(chan bool, numGoroutines)
		
		for g := 0; g < numGoroutines; g++ {
			go func() {
				for p := 0; p < packetsPerGoroutine; p++ {
					payload := make([]byte, 8192)
					packet := CreatePacket(uint64(p), payload, 0)
					SerializePacket(packet)
				}
				done <- true
			}()
		}
		
		// Wait for all goroutines to complete
		for g := 0; g < numGoroutines; g++ {
			<-done
		}
	}
}

// BenchmarkNetworkSimulation benchmarks network-like conditions
func BenchmarkNetworkSimulation(b *testing.B) {
	// Simulate network conditions with varying packet sizes
	packetSizes := []int{1024, 4096, 8192, 16384}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, size := range packetSizes {
			payload := make([]byte, size)
			rand.Read(payload)
			
			packet := CreatePacket(uint64(i), payload, 0)
			packetData := SerializePacket(packet)
			
			// Simulate network processing
			receivedPacket, _ := DeserializePacket(packetData)
			ValidatePacket(receivedPacket)
		}
	}
}

// BenchmarkEndToEndSimulation benchmarks end-to-end transfer simulation
func BenchmarkEndToEndSimulation(b *testing.B) {
	// Simulate a complete transfer scenario
	config := &StreamConfig{
		BufferSize: DefaultBufferSize,
		PacketSize: 8192,
		WindowSize: DefaultWindowSize,
		EnableFEC:  false,
	}
	
	// Create sender and receiver
	sender := NewSender(config)
	receiver := NewReceiver(config)
	
	// Generate test data
	testData := make([]byte, 1024*1024) // 1MB
	rand.Read(testData)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate packet creation and processing
		sequenceNum := uint64(0)
		bytesProcessed := 0
		
		for bytesProcessed < len(testData) {
			remaining := len(testData) - bytesProcessed
			chunkSize := remaining
			if chunkSize > config.PacketSize {
				chunkSize = config.PacketSize
			}
			
			payload := testData[bytesProcessed : bytesProcessed+chunkSize]
			packet := CreatePacket(sequenceNum, payload, 0)
			
			// Simulate network transfer
			packetData := SerializePacket(packet)
			receivedPacket, _ := DeserializePacket(packetData)
			ValidatePacket(receivedPacket)
			
			bytesProcessed += chunkSize
			sequenceNum++
		}
	}
	
	// Cleanup
	sender.Close()
	receiver.Close()
}

// BenchmarkThroughputConversion benchmarks throughput unit conversion
func BenchmarkThroughputConversion(b *testing.B) {
	bytesPerSecond := 1000000000.0 // 1 GB/s
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertToGbps(bytesPerSecond)
		ConvertFromGbps(8.0) // 8 Gbps
	}
}

// BenchmarkTimestampOperations benchmarks timestamp operations
func BenchmarkTimestampOperations(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		timestamp := GetCurrentTimestamp()
		TimestampToTime(timestamp)
	}
}

// BenchmarkRandomDataGeneration benchmarks random data generation
func BenchmarkRandomDataGeneration(b *testing.B) {
	sizes := []int{1024, 4096, 8192, 16384}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, size := range sizes {
			GenerateRandomBytes(size)
		}
	}
}

// BenchmarkPacketComparison benchmarks packet comparison operations
func BenchmarkPacketComparison(b *testing.B) {
	payload1 := make([]byte, 8192)
	payload2 := make([]byte, 8192)
	rand.Read(payload1)
	rand.Read(payload2)
	
	packet1 := CreatePacket(0, payload1, 0)
	packet2 := CreatePacket(1, payload2, 0)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Compare packet headers
		_ = packet1.Header.SequenceNum == packet2.Header.SequenceNum
		_ = packet1.Header.Length == packet2.Header.Length
		_ = packet1.Header.Checksum == packet2.Header.Checksum
		
		// Compare payloads
		_ = bytes.Equal(packet1.Payload, packet2.Payload)
	}
}