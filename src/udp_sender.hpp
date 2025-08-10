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

namespace hts {

class UDPSender {
public:
    explicit UDPSender(const StreamConfig& config);
    ~UDPSender();

    // Main interface
    ErrorCode connect();
    ErrorCode send_file();
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
    struct sockaddr_in remote_addr_;
    bool connected_;
    
    // Link monitoring
    LinkStatus link_status_;
    mutable std::mutex link_mutex_;
    std::chrono::steady_clock::time_point last_heartbeat_;
    
    // Flow control
    uint32_t window_size_;
    std::map<uint64_t, Packet> sent_packets_;
    std::map<uint64_t, bool> acked_packets_;
    mutable std::mutex window_mutex_;
    
    // Performance optimization
    std::queue<Packet> send_buffer_;
    std::queue<uint64_t> ack_buffer_;
    mutable std::mutex buffer_mutex_;
    std::condition_variable buffer_cv_;
    
    // Control
    std::atomic<bool> running_;
    std::thread ack_thread_;
    std::thread heartbeat_thread_;
    std::thread link_monitor_thread_;
    std::mutex control_mutex_;
    
    // File handling
    std::ifstream input_file_;
    uint64_t sequence_num_;
    
    // Methods
    ErrorCode send_packet(const Packet& packet);
    ErrorCode send_binary_file();
    ErrorCode send_pcap_file();
    
    // Background workers
    void ack_processor();
    void heartbeat_sender();
    void link_monitor();
    void check_link_status();
    ErrorCode attempt_resync();
    
    // Flow control
    bool wait_for_window_space();
    void start_retransmission_timer(uint64_t sequence_num);
    void process_ack(uint64_t sequence_num);
    
    // Statistics
    void update_stats(uint64_t bytes_sent, uint64_t packets_sent);
    void update_throughput();
    
    // Utility
    bool is_connected() const;
    void log_info(const std::string& message) const;
    void log_error(const std::string& message) const;
    void log_debug(const std::string& message) const;
};

} // namespace hts