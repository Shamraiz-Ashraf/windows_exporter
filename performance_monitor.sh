#!/bin/bash

# Performance Monitor Script
# Monitors throughput and system resources during file transfer

echo "=== Performance Monitor ==="

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to monitor system resources
monitor_resources() {
    local pid=$1
    local duration=$2
    
    print_status $BLUE "Monitoring system resources for ${duration}s..."
    
    # Monitor CPU, memory, and network usage
    for i in $(seq 1 $duration); do
        if [ -n "$pid" ] && kill -0 $pid 2>/dev/null; then
            # Get CPU usage
            cpu_usage=$(ps -p $pid -o %cpu --no-headers 2>/dev/null || echo "0")
            
            # Get memory usage
            mem_usage=$(ps -p $pid -o %mem --no-headers 2>/dev/null || echo "0")
            
            # Get network statistics
            net_stats=$(netstat -i 2>/dev/null | grep -E "(eth0|lo)" | awk '{print $3, $7}' | head -1 || echo "0 0")
            
            print_status $YELLOW "Time: ${i}s | CPU: ${cpu_usage}% | Memory: ${mem_usage}% | Network: ${net_stats}"
        else
            print_status $GREEN "Process completed"
            break
        fi
        sleep 1
    done
}

# Function to calculate throughput
calculate_throughput() {
    local file_size=$1
    local duration=$2
    
    if [ -n "$duration" ] && [ "$duration" != "0" ]; then
        throughput_mbps=$(echo "scale=2; $file_size * 8 / $duration / 1000000" | bc -l 2>/dev/null || echo "0")
        throughput_gbps=$(echo "scale=2; $throughput_mbps / 1000" | bc -l 2>/dev/null || echo "0")
        echo "${throughput_mbps} Mbps (${throughput_gbps} Gbps)"
    else
        echo "0 Mbps (0 Gbps)"
    fi
}

# Main test function
run_performance_test() {
    local file_size_mb=$1
    local test_name=$2
    
    print_status $BLUE "=== ${test_name} ==="
    
    # Create test file
    print_status $YELLOW "Creating ${file_size_mb}MB test file..."
    dd if=/dev/urandom of=test_perf_${file_size_mb}mb.bin bs=1M count=$file_size_mb 2>/dev/null
    file_size=$(stat -c%s test_perf_${file_size_mb}mb.bin)
    original_hash=$(sha256sum test_perf_${file_size_mb}mb.bin | cut -d' ' -f1)
    
    print_status $GREEN "Test file created: $(ls -lh test_perf_${file_size_mb}mb.bin)"
    
    # Start receiver
    print_status $YELLOW "Starting receiver..."
    ./bin/receiver -o received_perf_${file_size_mb}mb.bin -p 8087 -u -s 1024 -m &
    RECEIVER_PID=$!
    
    # Wait for receiver to start
    sleep 3
    
    # Start sender and monitor
    print_status $YELLOW "Starting sender..."
    start_time=$(date +%s.%N)
    ./bin/sender -i test_perf_${file_size_mb}mb.bin -r localhost -p 8087 -u -s 1024 -m &
    SENDER_PID=$!
    
    # Monitor resources for a reasonable time
    monitor_duration=$((file_size_mb / 10 + 5)) # Estimate based on file size
    monitor_resources $SENDER_PID $monitor_duration
    
    # Wait for completion
    wait $SENDER_PID
    end_time=$(date +%s.%N)
    
    # Stop receiver
    kill $RECEIVER_PID 2>/dev/null || true
    wait $RECEIVER_PID 2>/dev/null || true
    
    # Calculate performance
    duration=$(echo "$end_time - $start_time" | bc -l)
    throughput=$(calculate_throughput $file_size $duration)
    
    print_status $GREEN "Transfer completed in ${duration}s"
    print_status $GREEN "Throughput: ${throughput}"
    
    # Verify integrity
    received_hash=$(sha256sum received_perf_${file_size_mb}mb.bin | cut -d' ' -f1)
    if [ "$original_hash" = "$received_hash" ]; then
        print_status $GREEN "✅ Integrity: PASSED"
    else
        print_status $RED "❌ Integrity: FAILED"
        return 1
    fi
    
    # Cleanup
    rm -f test_perf_${file_size_mb}mb.bin received_perf_${file_size_mb}mb.bin
    
    echo "${test_name}: ${throughput} in ${duration}s" >> performance_results.txt
}

# Check if binaries exist
if [ ! -f "./bin/sender" ] || [ ! -f "./bin/receiver" ]; then
    print_status $RED "Error: Binaries not found"
    exit 1
fi

# Clear previous results
rm -f performance_results.txt

# Run performance tests with different file sizes
print_status $BLUE "Running performance tests..."

# Test 1: 100MB file
run_performance_test 100 "100MB Transfer"

# Test 2: 500MB file
run_performance_test 500 "500MB Transfer"

# Test 3: 1GB file
run_performance_test 1024 "1GB Transfer"

# Display results summary
print_status $GREEN ""
print_status $GREEN "=== Performance Test Results ==="
if [ -f "performance_results.txt" ]; then
    cat performance_results.txt
fi

print_status $GREEN ""
print_status $GREEN "🎉 Performance monitoring completed!"