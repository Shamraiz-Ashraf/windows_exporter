#include "udp_receiver.hpp"
#include "utils.cpp"
#include <iostream>
#include <fstream>
#include <cstring>
#include <algorithm>
#include <chrono>
#include <thread>
#include <filesystem>

namespace hts {

UDPReceiver::UDPReceiver(const StreamConfig& config)
    : config_(config)
    , stats_()
    , state_(StreamState::IDLE)
    , socket_fd_(-1)
    , listening_(false)
    , continuous_mode_(config.continuous_mode)
    , data_sink_(config.data_sink)
    , output_dir_(config.output_directory)
    , file_counter_(0)
    , expected_seq_(0)
    , running_(false) {
    
    stats_.start_time = std::chrono::steady_clock::now();
    link_status_.is_connected = false;
    link_status_.last_heartbeat = std::chrono::steady_clock::now();
    
    // Initialize RAM buffer for RAM data sink
    if (data_sink_ == "ram") {
        ram_buffer_.reserve(1024 * 1024); // 1MB initial capacity
    }
}

UDPReceiver::~UDPReceiver() {
    close();
}

ErrorCode UDPReceiver::start() {
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
    setsockopt(socket_fd_, SOL_SOCKET, SO_RCVBUF, &config_.buffer_size, sizeof(config_.buffer_size));
    setsockopt(socket_fd_, SOL_SOCKET, SO_SNDBUF, &config_.buffer_size, sizeof(config_.buffer_size));
    
    // Set up local address
    memset(&local_addr_, 0, sizeof(local_addr_));
    local_addr_.sin_family = AF_INET;
    local_addr_.sin_port = htons(config_.port);
    
    if (inet_pton(AF_INET, config_.local_addr.c_str(), &local_addr_.sin_addr) <= 0) {
        log_error("Invalid local address: " + config_.local_addr);
        close();
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    // Bind socket
    if (bind(socket_fd_, (struct sockaddr*)&local_addr_, sizeof(local_addr_)) < 0) {
        log_error("Failed to bind UDP socket: " + std::string(strerror(errno)));
        close();
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    // Initialize data sink
    ErrorCode result = initialize_data_sink();
    if (result != ErrorCode::SUCCESS) {
        log_error("Failed to initialize data sink");
        close();
        return result;
    }
    
    listening_ = true;
    state_ = StreamState::CONNECTED;
    link_status_.is_connected = true;
    last_heartbeat_ = std::chrono::steady_clock::now();
    
    log_info("UDP receiver listening on " + config_.local_addr + ":" + std::to_string(config_.port));
    
    // Start background workers
    running_ = true;
    
    if (config_.enable_link_monitoring) {
        link_monitor_thread_ = std::thread(&UDPReceiver::link_monitor, this);
    }
    
    receive_thread_ = std::thread(&UDPReceiver::packet_receiver, this);
    process_thread_ = std::thread(&UDPReceiver::packet_processor, this);
    
    return ErrorCode::SUCCESS;
}

ErrorCode UDPReceiver::receive_file() {
    if (state_ != StreamState::CONNECTED) {
        return ErrorCode::CONNECTION_CLOSED;
    }
    
    state_ = StreamState::TRANSFERRING;
    log_info("Starting UDP file reception");
    
    // Wait for completion or error
    while (running_ && state_ == StreamState::TRANSFERRING) {
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    
    if (state_ == StreamState::COMPLETED) {
        log_info("UDP file reception completed");
        stats_.end_time = std::chrono::steady_clock::now();
    }
    
    return (state_ == StreamState::COMPLETED) ? ErrorCode::SUCCESS : ErrorCode::STREAM_CLOSED;
}

ErrorCode UDPReceiver::initialize_data_sink() {
    if (data_sink_ == "disk") {
        if (continuous_mode_) {
            // Create output directory for continuous mode
            try {
                std::filesystem::create_directories(output_dir_);
            } catch (const std::exception& e) {
                log_error("Failed to create output directory: " + std::string(e.what()));
                return ErrorCode::DIRECTORY_NOT_FOUND;
            }
        } else {
            // Create single output file
            ErrorCode result = create_output_file();
            if (result != ErrorCode::SUCCESS) {
                return result;
            }
        }
    } else if (data_sink_ == "ram") {
        // RAM buffer already initialized in constructor
    } else if (data_sink_ == "none") {
        // No data sink, just process packets
    } else {
        log_error("Invalid data sink: " + data_sink_);
        return ErrorCode::INVALID_DATA_SINK;
    }
    
    return ErrorCode::SUCCESS;
}

ErrorCode UDPReceiver::create_output_file() {
    current_file_.open(config_.output_file, std::ios::binary);
    if (!current_file_.is_open()) {
        log_error("Failed to create output file: " + config_.output_file);
        return ErrorCode::FILE_ROTATION_FAILED;
    }
    
    // Write file header based on format
    if (config_.file_format == "pcap") {
        // Write pcap file header
        std::vector<uint8_t> header(24);
        
        // Magic number (little-endian)
        header[0] = 0xD4; header[1] = 0xC3; header[2] = 0xB2; header[3] = 0xA1;
        
        // Version
        header[4] = 0x02; header[5] = 0x00; // Major version 2
        header[6] = 0x04; header[7] = 0x00; // Minor version 4
        
        // Timezone and accuracy
        header[8] = 0x00; header[9] = 0x00; header[10] = 0x00; header[11] = 0x00; // Timezone
        header[12] = 0x00; header[13] = 0x00; header[14] = 0x00; header[15] = 0x00; // Accuracy
        
        // Snapshot length
        header[16] = 0xFF; header[17] = 0xFF; header[18] = 0x00; header[19] = 0x00; // 65535
        
        // Link layer type (Ethernet)
        header[20] = 0x01; header[21] = 0x00; header[22] = 0x00; header[23] = 0x00; // Ethernet
        
        current_file_.write(reinterpret_cast<const char*>(header.data()), header.size());
    }
    
    return ErrorCode::SUCCESS;
}

void UDPReceiver::packet_receiver() {
    std::vector<uint8_t> buffer(config_.udp_payload_size + PACKET_HEADER_SIZE);
    
    while (running_) {
        // Set receive timeout
        struct timeval tv;
        tv.tv_sec = 0;
        tv.tv_usec = 100000; // 100ms
        setsockopt(socket_fd_, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
        
        socklen_t addr_len = sizeof(local_addr_);
        struct sockaddr_in remote_addr;
        ssize_t received = recvfrom(socket_fd_, buffer.data(), buffer.size(), 0,
                                   (struct sockaddr*)&remote_addr, &addr_len);
        
        if (received < 0) {
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                // Timeout, continue
                continue;
            }
            log_error("Error reading UDP packet: " + std::string(strerror(errno)));
            break;
        }
        
        if (received == 0) {
            continue;
        }
        
        try {
            // Deserialize packet
            std::vector<uint8_t> packet_data(buffer.begin(), buffer.begin() + received);
            Packet packet = deserialize_packet(packet_data);
            
            // Validate packet
            if (!validate_packet(packet)) {
                log_error("Invalid UDP packet received");
                stats_.errors++;
                continue;
            }
            
            // Handle special packets
            if (packet.header.flags & FLAG_HEARTBEAT) {
                // Send ACK for heartbeat
                send_ack(packet.header.sequence_num, remote_addr);
                {
                    std::lock_guard<std::mutex> lock(link_mutex_);
                    last_heartbeat_ = std::chrono::steady_clock::now();
                }
                continue;
            }
            
            if (packet.header.flags & FLAG_END_OF_STREAM) {
                log_info("Received end-of-stream packet");
                state_ = StreamState::COMPLETED;
                break;
            }
            
            if (packet.header.flags & FLAG_RESYNC) {
                log_info("Received resync packet");
                send_ack(packet.header.sequence_num, remote_addr);
                continue;
            }
            
            // Add to processing queue
            {
                std::lock_guard<std::mutex> lock(buffer_mutex_);
                if (receive_buffer_.size() < config_.buffer_size) {
                    receive_buffer_.push(packet);
                    buffer_cv_.notify_one();
                } else {
                    log_debug("Receive buffer full, dropping packet");
                }
            }
            
        } catch (const std::exception& e) {
            log_error("Failed to deserialize UDP packet: " + std::string(e.what()));
            stats_.errors++;
        }
    }
}

void UDPReceiver::packet_processor() {
    while (running_) {
        Packet packet;
        bool has_packet = false;
        
        {
            std::unique_lock<std::mutex> lock(buffer_mutex_);
            buffer_cv_.wait(lock, [this] { return !receive_buffer_.empty() || !running_; });
            
            if (!running_) break;
            
            if (!receive_buffer_.empty()) {
                packet = receive_buffer_.front();
                receive_buffer_.pop();
                has_packet = true;
            }
        }
        
        if (has_packet) {
            ErrorCode result = process_packet(packet);
            if (result != ErrorCode::SUCCESS) {
                log_error("Failed to process UDP packet " + std::to_string(packet.header.sequence_num));
                stats_.errors++;
            }
        }
    }
}

ErrorCode UDPReceiver::process_packet(const Packet& packet) {
    // Check if this is the expected packet
    if (packet.header.sequence_num == expected_seq_) {
        // Write packet payload to data sink
        ErrorCode result = write_to_data_sink(packet);
        if (result != ErrorCode::SUCCESS) {
            return result;
        }
        
        // Update statistics
        stats_.packets_received++;
        stats_.bytes_received += packet.payload.size();
        expected_seq_++;
        
        // Check for out-of-order packets that can now be processed
        process_out_of_order_packets();
        
        // Progress logging
        if (stats_.packets_received % 1000 == 0) {
            update_throughput();
            double throughput_gbps = convert_to_gbps(stats_.throughput);
            log_info("Received " + std::to_string(stats_.packets_received) + 
                    " packets, Throughput: " + std::to_string(throughput_gbps) + " Gbps");
        }
    } else if (packet.header.sequence_num > expected_seq_) {
        // Out-of-order packet, store for later
        std::lock_guard<std::mutex> lock(reassembly_mutex_);
        received_packets_[packet.header.sequence_num] = packet;
    } else {
        // Duplicate packet, just ignore
    }
    
    return ErrorCode::SUCCESS;
}

void UDPReceiver::process_out_of_order_packets() {
    std::lock_guard<std::mutex> lock(reassembly_mutex_);
    
    while (true) {
        auto it = received_packets_.find(expected_seq_);
        if (it == received_packets_.end()) {
            break;
        }
        
        // Write packet payload to data sink
        ErrorCode result = write_to_data_sink(it->second);
        if (result != ErrorCode::SUCCESS) {
            log_error("Failed to write out-of-order packet " + std::to_string(expected_seq_));
        }
        
        // Update statistics
        stats_.packets_received++;
        stats_.bytes_received += it->second.payload.size();
        
        // Remove from map and increment expected sequence
        received_packets_.erase(it);
        expected_seq_++;
    }
}

ErrorCode UDPReceiver::write_to_data_sink(const Packet& packet) {
    if (data_sink_ == "disk") {
        return write_to_disk(packet);
    } else if (data_sink_ == "ram") {
        return write_to_ram(packet);
    } else if (data_sink_ == "none") {
        // Don't save data, just process
        return ErrorCode::SUCCESS;
    } else {
        return ErrorCode::INVALID_DATA_SINK;
    }
}

ErrorCode UDPReceiver::write_to_disk(const Packet& packet) {
    std::lock_guard<std::mutex> lock(file_mutex_);
    
    if (continuous_mode_) {
        // Check if we need to rotate files
        if (config_.enable_file_rotation && stats_.current_file_size >= config_.max_file_size) {
            ErrorCode result = rotate_file();
            if (result != ErrorCode::SUCCESS) {
                return result;
            }
        }
    }
    
    if (config_.file_format == "bin") {
        // Write raw payload for binary files
        current_file_.write(reinterpret_cast<const char*>(packet.payload.data()), packet.payload.size());
        stats_.current_file_size += packet.payload.size();
        
    } else if (config_.file_format == "pcap") {
        // Write pcap packet header and payload
        std::vector<uint8_t> header(16);
        uint32_t packet_len = static_cast<uint32_t>(packet.payload.size());
        
        // Timestamp
        auto now = std::chrono::system_clock::now();
        auto duration = now.time_since_epoch();
        uint32_t sec = std::chrono::duration_cast<std::chrono::seconds>(duration).count();
        uint32_t usec = std::chrono::duration_cast<std::chrono::microseconds>(duration).count() % 1000000;
        
        // Timestamp (little-endian)
        header[0] = (sec >> 0) & 0xFF; header[1] = (sec >> 8) & 0xFF;
        header[2] = (sec >> 16) & 0xFF; header[3] = (sec >> 24) & 0xFF;
        header[4] = (usec >> 0) & 0xFF; header[5] = (usec >> 8) & 0xFF;
        header[6] = (usec >> 16) & 0xFF; header[7] = (usec >> 24) & 0xFF;
        
        // Packet length (little-endian)
        header[8] = (packet_len >> 0) & 0xFF; header[9] = (packet_len >> 8) & 0xFF;
        header[10] = (packet_len >> 16) & 0xFF; header[11] = (packet_len >> 24) & 0xFF;
        header[12] = (packet_len >> 0) & 0xFF; header[13] = (packet_len >> 8) & 0xFF;
        header[14] = (packet_len >> 16) & 0xFF; header[15] = (packet_len >> 24) & 0xFF;
        
        // Write header
        current_file_.write(reinterpret_cast<const char*>(header.data()), header.size());
        
        // Write payload
        current_file_.write(reinterpret_cast<const char*>(packet.payload.data()), packet.payload.size());
        
        stats_.current_file_size += 16 + packet_len;
    }
    
    return ErrorCode::SUCCESS;
}

ErrorCode UDPReceiver::write_to_ram(const Packet& packet) {
    std::lock_guard<std::mutex> lock(ram_mutex_);
    
    // Append payload to RAM buffer
    ram_buffer_.insert(ram_buffer_.end(), packet.payload.begin(), packet.payload.end());
    
    // Optional: Limit RAM buffer size
    if (ram_buffer_.size() > 100 * 1024 * 1024) { // 100MB limit
        ram_buffer_.erase(ram_buffer_.begin(), ram_buffer_.begin() + packet.payload.size());
    }
    
    return ErrorCode::SUCCESS;
}

ErrorCode UDPReceiver::rotate_file() {
    if (current_file_.is_open()) {
        current_file_.close();
    }
    
    file_counter_++;
    std::string filename = "stream_" + std::to_string(file_counter_) + "_" + 
                          std::to_string(std::chrono::system_clock::now().time_since_epoch().count());
    
    if (config_.file_format == "pcap") {
        filename += ".pcap";
    } else {
        filename += ".bin";
    }
    
    std::string filepath = output_dir_ + "/" + filename;
    
    current_file_.open(filepath, std::ios::binary);
    if (!current_file_.is_open()) {
        log_error("Failed to create rotated file: " + filepath);
        return ErrorCode::FILE_ROTATION_FAILED;
    }
    
    // Write file header if needed
    if (config_.file_format == "pcap") {
        std::vector<uint8_t> header(24);
        
        // Magic number (little-endian)
        header[0] = 0xD4; header[1] = 0xC3; header[2] = 0xB2; header[3] = 0xA1;
        
        // Version
        header[4] = 0x02; header[5] = 0x00; // Major version 2
        header[6] = 0x04; header[7] = 0x00; // Minor version 4
        
        // Timezone and accuracy
        header[8] = 0x00; header[9] = 0x00; header[10] = 0x00; header[11] = 0x00; // Timezone
        header[12] = 0x00; header[13] = 0x00; header[14] = 0x00; header[15] = 0x00; // Accuracy
        
        // Snapshot length
        header[16] = 0xFF; header[17] = 0xFF; header[18] = 0x00; header[19] = 0x00; // 65535
        
        // Link layer type (Ethernet)
        header[20] = 0x01; header[21] = 0x00; header[22] = 0x00; header[23] = 0x00; // Ethernet
        
        current_file_.write(reinterpret_cast<const char*>(header.data()), header.size());
    }
    
    stats_.files_created++;
    stats_.current_file_size = 0;
    
    log_info("Rotated to new file: " + filename);
    return ErrorCode::SUCCESS;
}

void UDPReceiver::send_ack(uint64_t sequence_num, const struct sockaddr_in& remote_addr) {
    // Send ACK (non-blocking)
    std::thread([this, sequence_num, remote_addr]() {
        std::vector<uint8_t> ack_data(8);
        
        // Serialize sequence number (big-endian)
        for (int i = 0; i < 8; ++i) {
            ack_data[i] = (sequence_num >> (56 - i * 8)) & 0xFF;
        }
        
        // Set send timeout
        struct timeval tv;
        tv.tv_sec = 0;
        tv.tv_usec = 100000; // 100ms
        setsockopt(socket_fd_, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv));
        
        ssize_t sent = sendto(socket_fd_, ack_data.data(), ack_data.size(), 0,
                             (struct sockaddr*)&remote_addr, sizeof(remote_addr));
        
        if (sent < 0) {
            log_debug("Failed to send UDP ACK for packet " + std::to_string(sequence_num));
        }
    }).detach();
}

void UDPReceiver::link_monitor() {
    while (running_) {
        std::this_thread::sleep_for(config_.link_monitor_interval);
        
        if (!running_) break;
        
        check_link_status();
    }
}

void UDPReceiver::check_link_status() {
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
            log_info("Link restored");
            link_status_.is_connected = true;
            state_ = StreamState::TRANSFERRING;
        }
    }
}

StreamStats UDPReceiver::get_stats() const {
    std::lock_guard<std::mutex> lock(control_mutex_);
    StreamStats stats = stats_;
    stats.throughput = calculate_throughput(stats.bytes_received, 
        std::chrono::duration_cast<std::chrono::milliseconds>(
            std::chrono::steady_clock::now() - stats.start_time));
    stats.last_link_status = link_status_.is_connected;
    return stats;
}

void UDPReceiver::update_stats(uint64_t bytes_received, uint64_t packets_received) {
    std::lock_guard<std::mutex> lock(control_mutex_);
    stats_.bytes_received += bytes_received;
    stats_.packets_received += packets_received;
}

void UDPReceiver::update_throughput() {
    auto now = std::chrono::steady_clock::now();
    auto duration = std::chrono::duration_cast<std::chrono::milliseconds>(now - stats_.start_time);
    stats_.throughput = calculate_throughput(stats_.bytes_received, duration);
}

void UDPReceiver::close() {
    running_ = false;
    buffer_cv_.notify_all();
    
    if (receive_thread_.joinable()) {
        receive_thread_.join();
    }
    if (process_thread_.joinable()) {
        process_thread_.join();
    }
    if (link_monitor_thread_.joinable()) {
        link_monitor_thread_.join();
    }
    
    if (current_file_.is_open()) {
        current_file_.close();
    }
    
    if (socket_fd_ >= 0) {
        ::close(socket_fd_);
        socket_fd_ = -1;
    }
    
    listening_ = false;
    state_ = StreamState::IDLE;
    stats_.end_time = std::chrono::steady_clock::now();
}

bool UDPReceiver::is_listening() const {
    return listening_ && link_status_.is_connected;
}

void UDPReceiver::log_info(const std::string& message) const {
    std::cout << "[INFO] " << message << std::endl;
}

void UDPReceiver::log_error(const std::string& message) const {
    std::cerr << "[ERROR] " << message << std::endl;
}

void UDPReceiver::log_debug(const std::string& message) const {
    if (config_.log_level == "debug") {
        std::cout << "[DEBUG] " << message << std::endl;
    }
}

} // namespace hts