# High-Throughput Stream Sender/Receiver

A high-performance, zero-loss bit stream sender and receiver system designed for FEC (Forward Error Correction) projects with FPGA differential line transceivers. This system achieves minimum 7 Gbps throughput with byte-per-byte accuracy.

## Features

- **High Throughput**: Optimized for 7+ Gbps data transfer
- **Zero Loss**: Guaranteed byte-per-byte accuracy with retransmission and FEC
- **Multiple Formats**: Support for binary (.bin) and pcap files
- **Forward Error Correction**: Optional FEC with configurable redundancy
- **Flow Control**: Sliding window protocol for optimal performance
- **Real-time Monitoring**: Live throughput and progress statistics
- **Configurable**: Extensive configuration options for different use cases

## Architecture

The system consists of two main components:

### Sender
- Reads input files (binary or pcap format)
- Chunks data into optimal packet sizes
- Implements sliding window flow control
- Provides automatic retransmission
- Optional FEC encoding
- Real-time progress monitoring

### Receiver
- Accepts incoming connections
- Reassembles packets in correct order
- Handles out-of-order packet delivery
- Optional FEC decoding and recovery
- Writes data to output files
- Provides comprehensive statistics

## Performance Optimizations

- **Optimal Packet Sizing**: Calculates packet size based on bandwidth-delay product
- **Buffer Management**: Configurable buffer sizes for different network conditions
- **TCP Optimizations**: TCP_NODELAY, optimized buffer sizes
- **Concurrent Processing**: Multiple goroutines for I/O operations
- **Memory Efficiency**: Zero-copy operations where possible

## Installation

### Prerequisites

- Go 1.21 or later
- Linux (optimized for Linux networking stack)

### Build

```bash
# Clone the repository
git clone <repository-url>
cd high-throughput-stream

# Install dependencies
make deps

# Build binaries
make build
```

The binaries will be created in the `bin/` directory:
- `bin/sender` - High-throughput sender
- `bin/receiver` - High-throughput receiver

## Usage

### Basic Usage

1. **Start the receiver**:
```bash
./bin/receiver -output=received.bin -port=8080
```

2. **Start the sender**:
```bash
./bin/sender -input=test.bin -remote=localhost -port=8080
```

### Advanced Usage

#### With FEC (Forward Error Correction)
```bash
# Receiver with FEC
./bin/receiver -output=received.bin -port=8080 -fec

# Sender with FEC
./bin/sender -input=test.bin -remote=localhost -port=8080 -fec
```

#### With Custom Configuration
```bash
# Using configuration file
./bin/sender -config=config.yaml -input=test.bin
./bin/receiver -config=config.yaml -output=received.bin
```

#### PCAP File Transfer
```bash
# Transfer pcap files
./bin/sender -input=capture.pcap -format=pcap -remote=localhost -port=8080
./bin/receiver -output=received.pcap -format=pcap -port=8080
```

### Command Line Options

#### Sender Options
- `-input`: Input file path (required)
- `-remote`: Remote receiver address (default: localhost)
- `-port`: Port number (default: 8080)
- `-format`: File format: bin or pcap (default: bin)
- `-fec`: Enable Forward Error Correction
- `-config`: Configuration file path
- `-verbose`: Enable verbose logging

#### Receiver Options
- `-output`: Output file path (required)
- `-local`: Local address to bind to (default: 0.0.0.0)
- `-port`: Port number (default: 8080)
- `-format`: File format: bin or pcap (default: bin)
- `-fec`: Enable Forward Error Correction
- `-config`: Configuration file path
- `-verbose`: Enable verbose logging

## Configuration

The system can be configured using YAML configuration files. See `config.yaml` for a complete example.

### Key Configuration Sections

#### Network Configuration
```yaml
network:
  local_addr: "0.0.0.0"
  remote_addr: "localhost"
  port: 8080
```

#### Performance Configuration
```yaml
performance:
  buffer_size: 1048576  # 1MB buffer
  packet_size: 8192     # 8KB packets
  window_size: 1000     # Sliding window size
```

#### FEC Configuration
```yaml
fec:
  enable: false         # Enable FEC
  redundancy: 0.2       # 20% redundancy
```

#### Timing Configuration
```yaml
timing:
  timeout: 30s          # Connection timeout
  retry_interval: 100ms # Retransmission interval
  heartbeat_interval: 1s # Heartbeat interval
```

## Performance Tuning

### For Maximum Throughput

1. **Increase buffer sizes**:
```yaml
performance:
  buffer_size: 2097152  # 2MB
  packet_size: 16384    # 16KB
```

2. **Optimize for your network**:
```yaml
performance:
  window_size: 2000     # Larger window for high-latency networks
```

3. **Use FEC for noisy channels**:
```yaml
fec:
  enable: true
  redundancy: 0.3       # 30% redundancy for noisy channels
```

### For Low Latency

1. **Reduce packet sizes**:
```yaml
performance:
  packet_size: 4096     # Smaller packets
```

2. **Reduce retry intervals**:
```yaml
timing:
  retry_interval: 50ms  # Faster retransmission
```

## Testing

### Generate Test Data
```bash
make generate-test-data
```

### Run Performance Test
```bash
make perf-test
```

### Run Unit Tests
```bash
make test
```

### Run Benchmarks
```bash
make test-bench
```

## Monitoring and Statistics

The system provides real-time statistics including:

- **Throughput**: Current and average data transfer rate
- **Packet Statistics**: Sent, received, lost, retransmitted packets
- **FEC Statistics**: FEC packets sent and used
- **Error Counts**: Total errors encountered
- **Progress**: Transfer completion percentage

Example output:
```
=== Transfer Statistics ===
Duration: 2m15s
Bytes Sent: 1073741824
Packets Sent: 131072
Packets Retransmitted: 15
Average Throughput: 7.85 Gbps
Errors: 0
```

## FEC Implementation

The system implements a simplified XOR-based FEC scheme:

- **Encoding**: XOR-based parity packets
- **Decoding**: Packet recovery using available data and FEC packets
- **Configurable**: Redundancy levels from 10% to 50%
- **Block-based**: FEC operates on configurable block sizes

For production use, consider implementing more sophisticated FEC algorithms like:
- Reed-Solomon codes
- LDPC codes
- Raptor codes

## FPGA Integration

This system is designed to work with FPGA differential line transceivers:

1. **Sender** → **FPGA Transceiver** → **FPGA Transceiver** → **Receiver**
2. The FPGA handles the physical layer transmission
3. This system provides reliable transport layer with FEC

### FPGA Interface Considerations

- Use appropriate packet sizes for your FPGA buffer sizes
- Configure timing parameters based on FPGA processing delays
- Consider using FEC for noisy differential lines
- Monitor statistics for FPGA-induced packet loss

## Troubleshooting

### Common Issues

1. **Low Throughput**:
   - Check network interface speed
   - Increase buffer sizes
   - Optimize packet sizes
   - Check for network congestion

2. **Packet Loss**:
   - Enable FEC
   - Increase retry intervals
   - Check network stability
   - Monitor for FPGA issues

3. **Connection Issues**:
   - Verify firewall settings
   - Check port availability
   - Ensure correct IP addresses

### Debug Mode

Enable verbose logging for detailed debugging:
```bash
./bin/sender -input=test.bin -verbose
./bin/receiver -output=received.bin -verbose
```

## Development

### Project Structure
```
high-throughput-stream/
├── cmd/
│   ├── sender/          # Sender command-line application
│   └── receiver/        # Receiver command-line application
├── pkg/
│   └── stream/          # Core streaming library
│       ├── types.go     # Type definitions
│       ├── errors.go    # Error definitions
│       ├── utils.go     # Utility functions
│       ├── fec.go       # FEC implementation
│       ├── sender.go    # Sender implementation
│       └── receiver.go  # Receiver implementation
├── config.yaml          # Sample configuration
├── Makefile            # Build and test automation
├── go.mod              # Go module definition
└── README.md           # This file
```

### Building from Source
```bash
# Install dependencies
go mod download

# Build
go build -o bin/sender ./cmd/sender
go build -o bin/receiver ./cmd/receiver

# Run tests
go test ./pkg/stream/...

# Run benchmarks
go test -bench=. ./pkg/stream/...
```

## License

[Add your license information here]

## Contributing

[Add contribution guidelines here]

## Support

[Add support information here]