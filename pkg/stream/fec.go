package stream

import (
	"fmt"
	"math"
)

// FECBlock represents a block of data with FEC encoding
type FECBlock struct {
	BlockID     uint32
	DataPackets []*Packet
	FECPackets  []*Packet
	TotalPackets int
	DataPacketsCount int
	FECPacketsCount  int
}

// FECEncoder handles Forward Error Correction encoding
type FECEncoder struct {
	config     FECConfig
	blockID    uint32
	packetSize int
}

// FECDecoder handles Forward Error Correction decoding
type FECDecoder struct {
	config     FECConfig
	blocks     map[uint32]*FECBlock
	packetSize int
}

// NewFECEncoder creates a new FEC encoder
func NewFECEncoder(config FECConfig, packetSize int) *FECEncoder {
	return &FECEncoder{
		config:     config,
		blockID:    0,
		packetSize: packetSize,
	}
}

// NewFECDecoder creates a new FEC decoder
func NewFECDecoder(config FECConfig, packetSize int) *FECDecoder {
	return &FECDecoder{
		config:     config,
		blocks:     make(map[uint32]*FECBlock),
		packetSize: packetSize,
	}
}

// EncodeBlock encodes a block of data packets with FEC
func (e *FECEncoder) EncodeBlock(dataPackets []*Packet) (*FECBlock, error) {
	if len(dataPackets) == 0 {
		return nil, fmt.Errorf("no data packets to encode")
	}

	block := &FECBlock{
		BlockID:          e.blockID,
		DataPackets:      dataPackets,
		DataPacketsCount: len(dataPackets),
		TotalPackets:     len(dataPackets),
	}

	// Calculate number of FEC packets needed
	fecPacketsCount := int(math.Ceil(float64(len(dataPackets)) * e.config.Redundancy))
	block.FECPacketsCount = fecPacketsCount
	block.TotalPackets += fecPacketsCount

	// Generate FEC packets using Reed-Solomon like algorithm
	fecPackets, err := e.generateFECPackets(dataPackets, fecPacketsCount)
	if err != nil {
		return nil, err
	}

	block.FECPackets = fecPackets
	e.blockID++

	return block, nil
}

// generateFECPackets generates FEC packets using XOR-based encoding
func (e *FECEncoder) generateFECPackets(dataPackets []*Packet, fecCount int) ([]*Packet, error) {
	fecPackets := make([]*Packet, fecCount)
	
	// Use simple XOR-based FEC for demonstration
	// In a real implementation, you would use Reed-Solomon or LDPC codes
	
	for i := 0; i < fecCount; i++ {
		// Create FEC packet payload by XORing data packets
		fecPayload := make([]byte, e.packetSize)
		
		for _, dataPacket := range dataPackets {
			// XOR with data packet payload
			for k := 0; k < len(dataPacket.Payload) && k < len(fecPayload); k++ {
				fecPayload[k] ^= dataPacket.Payload[k]
			}
		}
		
		// Create FEC packet
		flags := uint16(FlagFEC)
		fecPacket := CreatePacket(
			uint64(len(dataPackets)+i), // Sequence number
			fecPayload,
			flags,
		)
		
		fecPackets[i] = fecPacket
	}
	
	return fecPackets, nil
}

// AddPacket adds a packet to the decoder's block
func (d *FECDecoder) AddPacket(packet *Packet) error {
	// Extract block ID from packet (simplified - in real implementation, 
	// block ID would be part of the packet header)
	blockID := uint32(packet.Header.SequenceNum / uint64(d.config.BlockSize))
	
	block, exists := d.blocks[blockID]
	if !exists {
		block = &FECBlock{
			BlockID:          blockID,
			DataPackets:      make([]*Packet, d.config.BlockSize),
			FECPackets:       make([]*Packet, 0),
			TotalPackets:     d.config.BlockSize,
			DataPacketsCount: d.config.BlockSize,
		}
		d.blocks[blockID] = block
	}
	
	// Add packet to appropriate slot
	if packet.Header.Flags&FlagFEC != 0 {
		// This is an FEC packet
		block.FECPackets = append(block.FECPackets, packet)
	} else {
		// This is a data packet
		seqInBlock := packet.Header.SequenceNum % uint64(d.config.BlockSize)
		if seqInBlock < uint64(len(block.DataPackets)) {
			block.DataPackets[seqInBlock] = packet
		}
	}
	
	return nil
}

// DecodeBlock attempts to decode a block using FEC
func (d *FECDecoder) DecodeBlock(blockID uint32) ([]*Packet, error) {
	block, exists := d.blocks[blockID]
	if !exists {
		return nil, fmt.Errorf("block %d not found", blockID)
	}
	
	// Count missing data packets
	missingCount := 0
	missingIndices := make([]int, 0)
	
	for i, packet := range block.DataPackets {
		if packet == nil {
			missingCount++
			missingIndices = append(missingIndices, i)
		}
	}
	
	// If no packets are missing, return the data packets
	if missingCount == 0 {
		result := make([]*Packet, 0, len(block.DataPackets))
		for _, packet := range block.DataPackets {
			if packet != nil {
				result = append(result, packet)
			}
		}
		return result, nil
	}
	
	// If we have enough FEC packets, try to recover missing packets
	if len(block.FECPackets) >= missingCount {
		recoveredPackets, err := d.recoverMissingPackets(block, missingIndices)
		if err != nil {
			return nil, err
		}
		
		// Fill in the missing packets
		for i, recoveredPacket := range recoveredPackets {
			block.DataPackets[missingIndices[i]] = recoveredPacket
		}
		
		// Return all data packets
		result := make([]*Packet, 0, len(block.DataPackets))
		for _, packet := range block.DataPackets {
			if packet != nil {
				result = append(result, packet)
			}
		}
		return result, nil
	}
	
	// Not enough FEC packets to recover
	return nil, ErrFECDecodeFailed
}

// recoverMissingPackets recovers missing packets using FEC
func (d *FECDecoder) recoverMissingPackets(block *FECBlock, missingIndices []int) ([]*Packet, error) {
	// This is a simplified recovery using XOR
	// In a real implementation, you would use proper Reed-Solomon or LDPC decoding
	
	recoveredPackets := make([]*Packet, len(missingIndices))
	
	for i, missingIndex := range missingIndices {
		// Use the first FEC packet to recover this missing packet
		if i < len(block.FECPackets) {
			fecPacket := block.FECPackets[i]
			
			// Recover by XORing FEC packet with all available data packets
			recoveredPayload := make([]byte, len(fecPacket.Payload))
			copy(recoveredPayload, fecPacket.Payload)
			
			for j, dataPacket := range block.DataPackets {
				if dataPacket != nil && j != missingIndex {
					// XOR with available data packet
					for k := 0; k < len(dataPacket.Payload) && k < len(recoveredPayload); k++ {
						recoveredPayload[k] ^= dataPacket.Payload[k]
					}
				}
			}
			
			// Create recovered packet
			recoveredPacket := CreatePacket(
				uint64(missingIndex),
				recoveredPayload,
				0, // No flags for recovered packet
			)
			
			recoveredPackets[i] = recoveredPacket
		}
	}
	
	return recoveredPackets, nil
}

// IsBlockComplete checks if a block has enough packets to be decoded
func (d *FECDecoder) IsBlockComplete(blockID uint32) bool {
	block, exists := d.blocks[blockID]
	if !exists {
		return false
	}
	
	// Count available data packets
	availableData := 0
	for _, packet := range block.DataPackets {
		if packet != nil {
			availableData++
		}
	}
	
	// Check if we have enough packets (data + FEC) to recover the block
	totalAvailable := availableData + len(block.FECPackets)
	return totalAvailable >= d.config.BlockSize
}

// GetBlockStats returns statistics about a block
func (d *FECDecoder) GetBlockStats(blockID uint32) (int, int, int) {
	block, exists := d.blocks[blockID]
	if !exists {
		return 0, 0, 0
	}
	
	availableData := 0
	for _, packet := range block.DataPackets {
		if packet != nil {
			availableData++
		}
	}
	
	return availableData, len(block.FECPackets), d.config.BlockSize
}

// CleanupBlock removes a block from memory after successful decoding
func (d *FECDecoder) CleanupBlock(blockID uint32) {
	delete(d.blocks, blockID)
}