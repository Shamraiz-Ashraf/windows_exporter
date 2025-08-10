#include "udp_receiver.hpp"
#include "utils.cpp"
#include <iostream>
#include <fstream>
#include <signal.h>
#include <cstring>
#include <chrono>
#include <thread>

using namespace hts;

// Global variables for signal handling
std::atomic<bool> running(true);
UDPReceiver* g_receiver = nullptr;

void signal_handler(int signal) {
    std::cout << "\nReceived signal " << signal << ", shutting down..." << std::endl;
    running = false;
    if (g_receiver) {
        g_receiver->close();
    }
}

void print_usage(const char* program_name) {
    std::cout << "Usage: " << program_name << " [OPTIONS]\n"
              << "Options:\n"
              << "  -o, --output FILE       Output file path\n"
              << "  -l, --local ADDR        Local address to bind to (default: 0.0.0.0)\n"
              << "  -p, --port PORT         Port number (default: 8080)\n"
              << "  -f, --format FORMAT     File format: bin or pcap (default: bin)\n"
              << "  -u, --udp               Use UDP instead of TCP\n"
              << "  -s, --udp-payload SIZE  UDP payload size in bytes (default: 1024)\n"
              << "  -m, --link-monitor      Enable link interruption monitoring\n"
              << "  -t, --link-timeout MS   Link timeout in milliseconds (default: 5000)\n"
              << "  -c, --continuous        Run in continuous mode\n"
              << "  -d, --data-sink SINK    Data sink: disk, ram, or none (default: disk)\n"
              << "  -o, --output-dir DIR    Output directory for continuous mode (default: ./output)\n"
              << "  -m, --max-file-size MB  Maximum file size in MB before rotation (default: 100)\n"
              << "  -R, --file-rotation     Enable file rotation in continuous mode\n"
              << "  -e, --fec               Enable Forward Error Correction\n"
              << "  -v, --verbose           Enable verbose logging\n"
              << "  -h, --help              Show this help message\n"
              << "\n"
              << "Examples:\n"
              << "  " << program_name << " -o received.bin -l 0.0.0.0 -p 8080 -u -s 1024\n"
              << "  " << program_name << " -o received.bin -u -m -e -v\n"
              << "  " << program_name << " -u -c -d ram\n"
              << std::endl;
}

int main(int argc, char* argv[]) {
    // Set up signal handling
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);
    
    // Parse command line arguments
    StreamConfig config;
    std::string output_file;
    
    for (int i = 1; i < argc; ++i) {
        std::string arg = argv[i];
        
        if (arg == "-h" || arg == "--help") {
            print_usage(argv[0]);
            return 0;
        } else if (arg == "-o" || arg == "--output") {
            if (++i < argc) {
                output_file = argv[i];
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-l" || arg == "--local") {
            if (++i < argc) {
                config.local_addr = argv[i];
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-p" || arg == "--port") {
            if (++i < argc) {
                config.port = static_cast<uint16_t>(std::stoi(argv[i]));
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-f" || arg == "--format") {
            if (++i < argc) {
                config.file_format = argv[i];
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-u" || arg == "--udp") {
            config.use_udp = true;
        } else if (arg == "-s" || arg == "--udp-payload") {
            if (++i < argc) {
                config.udp_payload_size = static_cast<uint32_t>(std::stoi(argv[i]));
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-m" || arg == "--link-monitor") {
            config.enable_link_monitoring = true;
        } else if (arg == "-t" || arg == "--link-timeout") {
            if (++i < argc) {
                config.link_timeout = std::chrono::milliseconds(std::stoi(argv[i]));
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-c" || arg == "--continuous") {
            config.continuous_mode = true;
        } else if (arg == "-d" || arg == "--data-sink") {
            if (++i < argc) {
                config.data_sink = argv[i];
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-o" || arg == "--output-dir") {
            if (++i < argc) {
                config.output_directory = argv[i];
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-m" || arg == "--max-file-size") {
            if (++i < argc) {
                config.max_file_size = static_cast<uint64_t>(std::stoi(argv[i])) * 1024 * 1024;
            } else {
                std::cerr << "Error: Missing argument for " << arg << std::endl;
                return 1;
            }
        } else if (arg == "-R" || arg == "--file-rotation") {
            config.enable_file_rotation = true;
        } else if (arg == "-e" || arg == "--fec") {
            config.enable_fec = true;
        } else if (arg == "-v" || arg == "--verbose") {
            config.log_level = "debug";
        } else {
            std::cerr << "Error: Unknown option " << arg << std::endl;
            print_usage(argv[0]);
            return 1;
        }
    }
    
    // Validate required parameters
    if (output_file.empty() && !config.continuous_mode) {
        std::cerr << "Error: Output file is required (unless in continuous mode)" << std::endl;
        print_usage(argv[0]);
        return 1;
    }
    
    if (!output_file.empty()) {
        config.output_file = output_file;
    }
    
    std::cout << "=== High-Throughput UDP Stream Receiver ===" << std::endl;
    std::cout << "Configuration:" << std::endl;
    std::cout << "  Local Address: " << config.local_addr << ":" << config.port << std::endl;
    std::cout << "  Protocol: " << (config.use_udp ? "UDP" : "TCP") << std::endl;
    if (config.use_udp) {
        std::cout << "  UDP Payload Size: " << config.udp_payload_size << " bytes" << std::endl;
    }
    std::cout << "  File Format: " << config.file_format << std::endl;
    std::cout << "  Link Monitoring: " << (config.enable_link_monitoring ? "Enabled" : "Disabled") << std::endl;
    std::cout << "  FEC: " << (config.enable_fec ? "Enabled" : "Disabled") << std::endl;
    std::cout << "  Continuous Mode: " << (config.continuous_mode ? "Enabled" : "Disabled") << std::endl;
    if (config.continuous_mode) {
        std::cout << "  Data Sink: " << config.data_sink << std::endl;
        std::cout << "  Output Directory: " << config.output_directory << std::endl;
    }
    if (!output_file.empty()) {
        std::cout << "  Output File: " << output_file << std::endl;
    }
    std::cout << std::endl;
    
    // Create UDP receiver
    UDPReceiver receiver(config);
    g_receiver = &receiver;
    
    // Start receiver
    std::cout << "Starting receiver..." << std::endl;
    ErrorCode result = receiver.start();
    if (result != ErrorCode::SUCCESS) {
        std::cerr << "Failed to start receiver: " << get_error_string(result) << std::endl;
        return 1;
    }
    
    // Start file reception in background
    std::thread receive_thread([&receiver]() {
        ErrorCode result = receiver.receive_file();
        if (result != ErrorCode::SUCCESS) {
            std::cerr << "Reception failed: " << get_error_string(result) << std::endl;
            running = false;
        }
    });
    
    // Monitor reception progress
    std::thread monitor_thread([&receiver]() {
        while (running) {
            std::this_thread::sleep_for(std::chrono::seconds(5));
            
            if (!running) break;
            
            StreamStats stats = receiver.get_stats();
            double throughput_gbps = convert_to_gbps(stats.throughput);
            
            std::cout << "Progress: " << stats.bytes_received << " bytes received, "
                      << std::fixed << std::setprecision(2) << throughput_gbps << " Gbps, "
                      << stats.packets_received << " packets received, "
                      << stats.packets_lost << " lost, "
                      << "Link: " << (stats.last_link_status ? "UP" : "DOWN") << std::endl;
        }
    });
    
    // Wait for completion or signal
    receive_thread.join();
    monitor_thread.join();
    
    // Print final statistics
    StreamStats final_stats = receiver.get_stats();
    auto duration = std::chrono::duration_cast<std::chrono::milliseconds>(
        final_stats.end_time - final_stats.start_time);
    double throughput_gbps = convert_to_gbps(final_stats.throughput);
    
    std::cout << "\n=== Reception Statistics ===" << std::endl;
    std::cout << "Duration: " << duration.count() << " ms" << std::endl;
    std::cout << "Bytes Received: " << final_stats.bytes_received << std::endl;
    std::cout << "Packets Received: " << final_stats.packets_received << std::endl;
    std::cout << "Packets Lost: " << final_stats.packets_lost << std::endl;
    std::cout << "Average Throughput: " << std::fixed << std::setprecision(2) 
              << throughput_gbps << " Gbps" << std::endl;
    std::cout << "Errors: " << final_stats.errors << std::endl;
    std::cout << "Link Interruptions: " << final_stats.link_interruptions << std::endl;
    std::cout << "Files Created: " << final_stats.files_created << std::endl;
    
    if (final_stats.fec_packets_used > 0) {
        std::cout << "FEC Packets Used: " << final_stats.fec_packets_used << std::endl;
    }
    
    std::cout << "Receiver shutdown complete" << std::endl;
    return 0;
}