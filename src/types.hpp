#pragma once

#include <cstdint>
#include <string>
#include <vector>
#include <chrono>
#include <memory>

namespace hts {

// Packet header structure (32 bytes)
struct PacketHeader {
    uint32_t magic;           // Magic number for packet identification
    uint64_t sequence_num;    // Sequence number for ordering
    uint32_t length;          // Length of payload
    uint32_t checksum;        // CRC32 checksum of payload
    uint64_t timestamp;       // Timestamp when packet was created
    uint16_t flags;           // Various flags (FEC, retransmission, etc.)
    uint16_t reserved;        // Reserved for future use
} __attribute__((packed));

// Packet flags
constexpr uint16_t FLAG_FEC = 0x0001;
constexpr uint16_t FLAG_RETRANSMIT = 0x0002;
constexpr uint16_t FLAG_HEARTBEAT = 0x0004;
constexpr uint16_t FLAG_END_OF_STREAM = 0x0008;
constexpr uint16_t FLAG_COMPRESSED = 0x0010;
constexpr uint16_t FLAG_LINK_STATUS = 0x0020;
constexpr uint16_t FLAG_RESYNC = 0x0040;
constexpr uint16_t FLAG_UDP_PAYLOAD = 0x0080;

// Constants
constexpr uint32_t PACKET_HEADER_SIZE = 32;
constexpr uint32_t DEFAULT_MAGIC = 0xDEADBEEF;
constexpr uint32_t MAX_PACKET_SIZE = 65536;
constexpr uint32_t DEFAULT_BUFFER_SIZE = 1024 * 1024; // 1MB
constexpr uint32_t DEFAULT_WINDOW_SIZE = 1000;
constexpr uint32_t DEFAULT_UDP_PAYLOAD_SIZE = 1024; // 1024 bytes as required

// Complete packet structure
struct Packet {
    PacketHeader header;
    std::vector<uint8_t> payload;
};

// Stream configuration
struct StreamConfig {
    // Network configuration
    std::string local_addr = "0.0.0.0";
    std::string remote_addr = "localhost";
    uint16_t port = 8080;
    bool use_udp = false;
    
    // Performance configuration
    uint32_t buffer_size = DEFAULT_BUFFER_SIZE;
    uint32_t packet_size = 8192;
    uint32_t window_size = DEFAULT_WINDOW_SIZE;
    uint32_t udp_payload_size = DEFAULT_UDP_PAYLOAD_SIZE;
    
    // FEC configuration
    bool enable_fec = false;
    double fec_redundancy = 0.2;
    
    // Timing configuration
    std::chrono::milliseconds timeout{30000};
    std::chrono::milliseconds retry_interval{100};
    std::chrono::milliseconds heartbeat_interval{1000};
    
    // Link monitoring configuration
    std::chrono::milliseconds link_monitor_interval{1000};
    std::chrono::milliseconds link_timeout{5000};
    bool enable_link_monitoring = false;
    
    // Continuous mode configuration
    bool continuous_mode = false;
    std::string data_sink = "disk"; // "disk", "ram", "none"
    std::string output_directory = "./output";
    uint64_t max_file_size = 100 * 1024 * 1024; // 100MB
    bool enable_file_rotation = false;
    
    // File configuration
    std::string input_file;
    std::string output_file;
    std::string file_format = "bin"; // "bin" or "pcap"
    
    // Logging
    std::string log_level = "info";
    bool enable_metrics = true;
};

// Stream statistics
struct StreamStats {
    std::chrono::steady_clock::time_point start_time;
    std::chrono::steady_clock::time_point end_time;
    uint64_t bytes_sent = 0;
    uint64_t bytes_received = 0;
    uint64_t packets_sent = 0;
    uint64_t packets_received = 0;
    uint64_t packets_lost = 0;
    uint64_t packets_retransmitted = 0;
    uint64_t fec_packets_sent = 0;
    uint64_t fec_packets_used = 0;
    double throughput = 0.0; // bytes per second
    std::chrono::microseconds latency{0};
    uint64_t errors = 0;
    
    // Link monitoring stats
    uint64_t link_interruptions = 0;
    bool last_link_status = false;
    uint64_t resync_count = 0;
    
    // Continuous mode stats
    uint64_t files_created = 0;
    uint64_t current_file_size = 0;
};

// Stream state enumeration
enum class StreamState {
    IDLE,
    CONNECTING,
    CONNECTED,
    TRANSFERRING,
    LINK_INTERRUPTED,
    RESYNCING,
    COMPLETED,
    ERROR
};

// FEC configuration
struct FECConfig {
    std::string algorithm = "xor";
    uint32_t block_size = 100;
    double redundancy = 0.2;
    uint32_t max_errors = 10;
};

// Link status
struct LinkStatus {
    bool is_connected = false;
    std::chrono::steady_clock::time_point last_heartbeat;
    uint64_t interruptions = 0;
    std::chrono::steady_clock::time_point last_resync;
};

// Error codes
enum class ErrorCode {
    SUCCESS = 0,
    INVALID_PACKET_SIZE,
    INVALID_MAGIC,
    PACKET_TOO_LARGE,
    CHECKSUM_MISMATCH,
    TIMEOUT,
    CONNECTION_CLOSED,
    INVALID_SEQUENCE,
    BUFFER_FULL,
    FEC_DECODE_FAILED,
    FILE_NOT_FOUND,
    INVALID_FILE_FORMAT,
    STREAM_CLOSED,
    LINK_INTERRUPTED,
    LINK_TIMEOUT,
    RESYNC_FAILED,
    INVALID_UDP_PAYLOAD,
    INVALID_DATA_SINK,
    FILE_ROTATION_FAILED,
    DIRECTORY_NOT_FOUND
};

// Utility functions
uint32_t calculate_crc32(const std::vector<uint8_t>& data);
std::vector<uint8_t> serialize_packet(const Packet& packet);
Packet deserialize_packet(const std::vector<uint8_t>& data);
bool validate_packet(const Packet& packet);
uint64_t get_current_timestamp();
double calculate_throughput(uint64_t bytes, std::chrono::milliseconds duration);
double convert_to_gbps(double bytes_per_second);
double convert_from_gbps(double gbps);

} // namespace hts