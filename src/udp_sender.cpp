#include "udp_sender.hpp"
#include "utils.cpp"
#include <iostream>
#include <fstream>
#include <cstring>
#include <algorithm>
#include <chrono>
#include <thread>

namespace hts {

UDPSender::UDPSender(const StreamConfig& config)
    : config_(config)
    , stats_()
    , state_(StreamState::IDLE)
    , socket_fd_(-1)
    , connected_(false)
    , window_size_(config.window_size)
    , running_(false)
    , sequence_num_(0) {
    
    stats_.start_time = std::chrono::steady_clock::now();
    link_status_.is_connected = false;
    link_status_.last_heartbeat = std::chrono::steady_clock::now();
}

UDPSender::~UDPSender() {
    close();
}

ErrorCode UDPSender::connect() {
    if (state_ != StreamState::IDLE) {
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    state_ = StreamState::CONNECTING;
    
    // Create UDP socket
    socket_fd_ = socket(AF_INET, SOCK_DGRAM, 0);
    if (socket_fd_ < 0) {
        log_error("Failed to create UDP socket");
        state_ = StreamState::ERROR;
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    // Set socket options for high performance
    int optval = 1;
    setsockopt(socket_fd_, SOL_SOCKET, SO_REUSEADDR, &optval, sizeof(optval));
    
    // Set buffer sizes
    setsockopt(socket_fd_, SOL_SOCKET, SO_SNDBUF, &config_.buffer_size, sizeof(config_.buffer_size));
    setsockopt(socket_fd_, SOL_SOCKET, SO_RCVBUF, &config_.buffer_size, sizeof(config_.buffer_size));
    
    // Set up remote address
    memset(&remote_addr_, 0, sizeof(remote_addr_));
    remote_addr_.sin_family = AF_INET;
    remote_addr_.sin_port = htons(config_.port);
    
    if (inet_pton(AF_INET, config_.remote_addr.c_str(), &remote_addr_.sin_addr) <= 0) {
        log_error("Invalid remote address: " + config_.remote_addr);
        close();
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    // Bind to local address
    struct sockaddr_in local_addr;
    memset(&local_addr, 0, sizeof(local_addr));
    local_addr.sin_family = AF_INET;
    local_addr.sin_addr.s_addr = htonl(INADDR_ANY);
    local_addr.sin_port = htons(0); // Let OS choose port
    
    if (bind(socket_fd_, (struct sockaddr*)&local_addr, sizeof(local_addr)) < 0) {
        log_error("Failed to bind UDP socket");
        close();
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    connected_ = true;
    state_ = StreamState::CONNECTED;
    link_status_.is_connected = true;
    last_heartbeat_ = std::chrono::steady_clock::now();
    
    log_info("UDP connection established to " + config_.remote_addr + ":" + std::to_string(config_.port));
    
    // Start background workers
    running_ = true;
    
    if (config_.enable_link_monitoring) {
        link_monitor_thread_ = std::thread(&UDPSender::link_monitor, this);
    }
    
    heartbeat_thread_ = std::thread(&UDPSender::heartbeat_sender, this);
    ack_thread_ = std::thread(&UDPSender::ack_processor, this);
    
    return ErrorCode::SUCCESS;
}

ErrorCode UDPSender::send_file() {
    if (state_ != StreamState::CONNECTED) {
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    state_ = StreamState::TRANSFERRING;
    log_info("Starting UDP file transfer");
    
    // Open input file
    input_file_.open(config_.input_file, std::ios::binary);
    if (!input_file_.is_open()) {
        log_error("Failed to open input file: " + config_.input_file);
        return ErrorCode::FILE_NOT_FOUND;
    }
    
    // Send file based on format
    ErrorCode result;
    if (config_.file_format == "bin") {
        result = send_binary_file();
    } else if (config_.file_format == "pcap") {
        result = send_pcap_file();
    } else {
        log_error("Invalid file format: " + config_.file_format);
        return ErrorCode::INVALID_FILE_FORMAT;
    }
    
    input_file_.close();
    
    if (result == ErrorCode::SUCCESS) {
        // Send end-of-stream packet
        Packet end_packet = create_packet(sequence_num_++, std::vector<uint8_t>(), FLAG_END_OF_STREAM | FLAG_UDP_PAYLOAD);
        send_packet(end_packet);
        
        log_info("UDP file transfer completed");
        state_ = StreamState::COMPLETED;
        stats_.end_time = std::chrono::steady_clock::now();
    }
    
    return result;
}

ErrorCode UDPSender::send_binary_file() {
    // Get file size for progress tracking
    input_file_.seekg(0, std::ios::end);
    uint64_t total_bytes = input_file_.tellg();
    input_file_.seekg(0, std::ios::beg);
    
    log_info("Sending binary file via UDP: " + config_.input_file + " (" + std::to_string(total_bytes) + " bytes)");
    
    // Use fixed 1024-byte payload size for UDP
    uint32_t payload_size = config_.udp_payload_size;
    if (payload_size == 0) {
        payload_size = DEFAULT_UDP_PAYLOAD_SIZE;
    }
    
    std::vector<uint8_t> buffer(payload_size);
    uint64_t bytes_sent = 0;
    
    while (input_file_.good() && running_) {
        // Read chunk from file
        input_file_.read(reinterpret_cast<char*>(buffer.data()), payload_size);
        std::streamsize bytes_read = input_file_.gcount();
        
        if (bytes_read == 0) {
            break;
        }
        
        // Create payload (pad to 1024 bytes if needed)
        std::vector<uint8_t> payload(buffer.begin(), buffer.begin() + bytes_read);
        if (payload.size() < payload_size) {
            payload.resize(payload_size, 0);
        }
        
        // Create packet
        uint16_t flags = FLAG_UDP_PAYLOAD;
        if (sequence_num_ == 0) {
            flags |= FLAG_COMPRESSED;
        }
        
        Packet packet = create_packet(sequence_num_++, payload, flags);
        
        // Send packet
        ErrorCode result = send_packet(packet);
        if (result != ErrorCode::SUCCESS) {
            log_error("Failed to send packet " + std::to_string(sequence_num_ - 1));
            return result;
        }
        
        bytes_sent += bytes_read;
        
        // Progress logging
        if (sequence_num_ % 1000 == 0) {
            double progress = static_cast<double>(bytes_sent) / total_bytes * 100.0;
            update_throughput();
            double throughput_gbps = convert_to_gbps(stats_.throughput);
            log_info("Progress: " + std::to_string(progress) + "%, Throughput: " + std::to_string(throughput_gbps) + " Gbps");
        }
    }
    
    return ErrorCode::SUCCESS;
}

ErrorCode UDPSender::send_pcap_file() {
    // Simplified pcap file sending - similar to binary for now
    return send_binary_file();
}

ErrorCode UDPSender::send_packet(const Packet& packet) {
    // Wait for window space
    if (!wait_for_window_space()) {
        return ErrorCode::BUFFER_FULL;
    }
    
    // Serialize packet
    std::vector<uint8_t> packet_data = serialize_packet(packet);
    
    // Send packet via UDP
    ssize_t sent = sendto(socket_fd_, packet_data.data(), packet_data.size(), 0,
                         (struct sockaddr*)&remote_addr_, sizeof(remote_addr_));
    
    if (sent < 0) {
        log_error("Failed to send UDP packet: " + std::string(strerror(errno)));
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    if (static_cast<size_t>(sent) != packet_data.size()) {
        log_error("Incomplete UDP packet sent");
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    // Track sent packet for flow control
    {
        std::lock_guard<std::mutex> lock(window_mutex_);
        sent_packets_[packet.header.sequence_num] = packet;
    }
    
    // Start retransmission timer
    start_retransmission_timer(packet.header.sequence_num);
    
    return ErrorCode::SUCCESS;
}

void UDPSender::ack_processor() {
    std::vector<uint8_t> buffer(1024);
    
    while (running_) {
        // Set receive timeout
        struct timeval tv;
        tv.tv_sec = 0;
        tv.tv_usec = 100000; // 100ms
        setsockopt(socket_fd_, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
        
        socklen_t addr_len = sizeof(remote_addr_);
        ssize_t received = recvfrom(socket_fd_, buffer.data(), buffer.size(), 0,
                                   (struct sockaddr*)&remote_addr_, &addr_len);
        
        if (received < 0) {
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                // Timeout, continue
                continue;
            }
            log_error("Error reading UDP ACK: " + std::string(strerror(errno)));
            break;
        }
        
        // Process ACK data
        if (received >= 8) {
            uint64_t ack_num = 0;
            for (int i = 0; i < 8; ++i) {
                ack_num = (ack_num << 8) | static_cast<uint8_t>(buffer[i]);
            }
            
            process_ack(ack_num);
        }
    }
}

void UDPSender::heartbeat_sender() {
    while (running_) {
        std::this_thread::sleep_for(config_.heartbeat_interval);
        
        if (!running_) break;
        
        Packet heartbeat = create_packet(0, std::vector<uint8_t>(), FLAG_HEARTBEAT | FLAG_UDP_PAYLOAD);
        send_packet(heartbeat);
        
        {
            std::lock_guard<std::mutex> lock(link_mutex_);
            last_heartbeat_ = std::chrono::steady_clock::now();
        }
    }
}

void UDPSender::link_monitor() {
    while (running_) {
        std::this_thread::sleep_for(config_.link_monitor_interval);
        
        if (!running_) break;
        
        check_link_status();
    }
}

void UDPSender::check_link_status() {
    std::lock_guard<std::mutex> lock(link_mutex_);
    
    auto time_since_heartbeat = std::chrono::steady_clock::now() - last_heartbeat_;
    if (time_since_heartbeat > config_.link_timeout) {
        if (link_status_.is_connected) {
            log_info("Link interruption detected");
            state_ = StreamState::LINK_INTERRUPTED;
            link_status_.is_connected = false;
            link_status_.interruptions++;
            stats_.link_interruptions++;
        }
    } else {
        if (!link_status_.is_connected) {
            log_info("Link restored, attempting resync");
            attempt_resync();
        }
    }
}

ErrorCode UDPSender::attempt_resync() {
    state_ = StreamState::RESYNCING;
    
    // Send resync packet
    Packet resync_packet = create_packet(0, std::vector<uint8_t>(), FLAG_RESYNC | FLAG_UDP_PAYLOAD);
    ErrorCode result = send_packet(resync_packet);
    
    if (result == ErrorCode::SUCCESS) {
        // Wait for acknowledgment
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        
        link_status_.is_connected = true;
        link_status_.last_resync = std::chrono::steady_clock::now();
        stats_.resync_count++;
        state_ = StreamState::TRANSFERRING;
        
        log_info("Resync completed successfully");
    }
    
    return result;
}

bool UDPSender::wait_for_window_space() {
    while (running_) {
        std::lock_guard<std::mutex> lock(window_mutex_);
        if (sent_packets_.size() < window_size_) {
            return true;
        }
        
        // Wait a bit before checking again
        std::this_thread::sleep_for(std::chrono::microseconds(100));
    }
    return false;
}

void UDPSender::start_retransmission_timer(uint64_t sequence_num) {
    // Start a timer for packet retransmission
    std::thread([this, sequence_num]() {
        std::this_thread::sleep_for(config_.retry_interval);
        
        if (!running_) return;
        
        // Check if packet was acknowledged
        bool acked = false;
        {
            std::lock_guard<std::mutex> lock(window_mutex_);
            acked = acked_packets_.find(sequence_num) != acked_packets_.end();
        }
        
        if (!acked) {
            // Retransmit packet
            std::lock_guard<std::mutex> lock(window_mutex_);
            auto it = sent_packets_.find(sequence_num);
            if (it != sent_packets_.end()) {
                log_debug("Retransmitting UDP packet " + std::to_string(sequence_num));
                Packet& packet = it->second;
                packet.header.flags |= FLAG_RETRANSMIT;
                send_packet(packet);
                stats_.packets_retransmitted++;
            }
        }
    }).detach();
}

void UDPSender::process_ack(uint64_t sequence_num) {
    std::lock_guard<std::mutex> lock(window_mutex_);
    acked_packets_[sequence_num] = true;
    sent_packets_.erase(sequence_num);
}

StreamStats UDPSender::get_stats() const {
    std::lock_guard<std::mutex> lock(control_mutex_);
    StreamStats stats = stats_;
    stats.throughput = calculate_throughput(stats.bytes_sent, 
        std::chrono::duration_cast<std::chrono::milliseconds>(
            std::chrono::steady_clock::now() - stats.start_time));
    stats.last_link_status = link_status_.is_connected;
    return stats;
}

void UDPSender::update_stats(uint64_t bytes_sent, uint64_t packets_sent) {
    std::lock_guard<std::mutex> lock(control_mutex_);
    stats_.bytes_sent += bytes_sent;
    stats_.packets_sent += packets_sent;
}

void UDPSender::update_throughput() {
    auto now = std::chrono::steady_clock::now();
    auto duration = std::chrono::duration_cast<std::chrono::milliseconds>(now - stats_.start_time);
    stats_.throughput = calculate_throughput(stats_.bytes_sent, duration);
}

void UDPSender::close() {
    running_ = false;
    
    if (ack_thread_.joinable()) {
        ack_thread_.join();
    }
    if (heartbeat_thread_.joinable()) {
        heartbeat_thread_.join();
    }
    if (link_monitor_thread_.joinable()) {
        link_monitor_thread_.join();
    }
    
    if (socket_fd_ >= 0) {
        ::close(socket_fd_);
        socket_fd_ = -1;
    }
    
    connected_ = false;
    state_ = StreamState::IDLE;
}

bool UDPSender::is_connected() const {
    return connected_ && link_status_.is_connected;
}

void UDPSender::log_info(const std::string& message) const {
    std::cout << "[INFO] " << message << std::endl;
}

void UDPSender::log_error(const std::string& message) const {
    std::cerr << "[ERROR] " << message << std::endl;
}

void UDPSender::log_debug(const std::string& message) const {
    if (config_.log_level == "debug") {
        std::cout << "[DEBUG] " << message << std::endl;
    }
}

} // namespace hts