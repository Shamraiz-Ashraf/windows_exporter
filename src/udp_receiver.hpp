#pragma once

#include "types.hpp"
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <thread>
#include <mutex>
#include <condition_variable>
#include <queue>
#include <atomic>
#include <memory>
#include <functional>
#include <fstream>
#include <map>

namespace hts {

class UDPReceiver {
public:
    explicit UDPReceiver(const StreamConfig& config);
    ~UDPReceiver();

    // Main interface
    ErrorCode start();
    ErrorCode receive_file();
    StreamStats get_stats() const;
    void close();

    // Control
    void set_running(bool running) { running_ = running; }
    bool is_running() const { return running_; }

private:
    // Configuration
    StreamConfig config_;
    StreamStats stats_;
    StreamState state_;
    
    // Network
    int socket_fd_;
    struct sockaddr_in local_addr_;
    bool listening_;
    
    // Link monitoring
    LinkStatus link_status_;
    mutable std::mutex link_mutex_;
    std::chrono::steady_clock::time_point last_heartbeat_;
    
    // Continuous mode
    bool continuous_mode_;
    std::string data_sink_;
    std::string output_dir_;
    std::ofstream current_file_;
    mutable std::mutex file_mutex_;
    uint64_t file_counter_;
    std::vector<uint8_t> ram_buffer_; // For RAM data sink
    mutable std::mutex ram_mutex_;
    
    // Packet reassembly
    std::map<uint64_t, Packet> received_packets_;
    uint64_t expected_seq_;
    mutable std::mutex reassembly_mutex_;
    
    // Performance optimization
    std::queue<Packet> receive_buffer_;
    mutable std::mutex buffer_mutex_;
    std::condition_variable buffer_cv_;
    
    // Control
    std::atomic<bool> running_;
    std::thread receive_thread_;
    std::thread process_thread_;
    std::thread link_monitor_thread_;
    std::mutex control_mutex_;
    
    // Methods
    ErrorCode initialize_data_sink();
    ErrorCode create_output_file();
    ErrorCode write_to_data_sink(const Packet& packet);
    ErrorCode write_to_disk(const Packet& packet);
    ErrorCode write_to_ram(const Packet& packet);
    ErrorCode rotate_file();
    
    // Background workers
    void packet_receiver();
    void packet_processor();
    void link_monitor();
    void check_link_status();
    
    // Packet handling
    ErrorCode process_packet(const Packet& packet);
    void process_out_of_order_packets();
    void send_ack(uint64_t sequence_num, const struct sockaddr_in& remote_addr);
    
    // Statistics
    void update_stats(uint64_t bytes_received, uint64_t packets_received);
    void update_throughput();
    
    // Utility
    bool is_listening() const;
    void log_info(const std::string& message) const;
    void log_error(const std::string& message) const;
    void log_debug(const std::string& message) const;
};

} // namespace hts