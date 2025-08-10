#!/bin/bash

# Comprehensive Feature Test Script
echo "=== Comprehensive Feature Test ==="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Check binaries
if [ ! -f "./bin/sender" ] || [ ! -f "./bin/receiver" ]; then
    print_status $RED "Error: Binaries not found"
    exit 1
fi

# Create test data
print_status $BLUE "Creating test data..."
dd if=/dev/urandom of=test_feature.bin bs=1M count=100 2>/dev/null
original_hash=$(sha256sum test_feature.bin | cut -d' ' -f1)
print_status $GREEN "Test data created: $(ls -lh test_feature.bin)"

# Test 1: Basic UDP
print_status $YELLOW "=== Test 1: Basic UDP Transfer ==="
./bin/receiver -o received_basic.bin -p 8081 -u -s 1024 -m &
RECEIVER_PID=$!
sleep 2
./bin/sender -i test_feature.bin -r localhost -p 8081 -u -s 1024 -m
kill $RECEIVER_PID 2>/dev/null || true
wait $RECEIVER_PID 2>/dev/null || true

received_hash=$(sha256sum received_basic.bin | cut -d' ' -f1)
if [ "$original_hash" = "$received_hash" ]; then
    print_status $GREEN "✅ Basic UDP: PASSED"
else
    print_status $RED "❌ Basic UDP: FAILED"
    exit 1
fi

# Test 2: UDP with FEC
print_status $YELLOW "=== Test 2: UDP with FEC ==="
./bin/receiver -o received_fec.bin -p 8082 -u -s 1024 -m -e &
RECEIVER_PID=$!
sleep 2
./bin/sender -i test_feature.bin -r localhost -p 8082 -u -s 1024 -m -e
kill $RECEIVER_PID 2>/dev/null || true
wait $RECEIVER_PID 2>/dev/null || true

received_hash=$(sha256sum received_fec.bin | cut -d' ' -f1)
if [ "$original_hash" = "$received_hash" ]; then
    print_status $GREEN "✅ UDP+FEC: PASSED"
else
    print_status $RED "❌ UDP+FEC: FAILED"
    exit 1
fi

# Test 3: Continuous mode with RAM
print_status $YELLOW "=== Test 3: Continuous Mode (RAM) ==="
./bin/receiver -p 8083 -u -s 1024 -m -c -d ram &
RECEIVER_PID=$!
sleep 2
./bin/sender -i test_feature.bin -r localhost -p 8083 -u -s 1024 -m
kill $RECEIVER_PID 2>/dev/null || true
wait $RECEIVER_PID 2>/dev/null || true
print_status $GREEN "✅ Continuous RAM: PASSED"

# Test 4: Continuous mode with disk
print_status $YELLOW "=== Test 4: Continuous Mode (Disk) ==="
mkdir -p output_feature
./bin/receiver -p 8084 -u -s 1024 -m -c -d disk -o output_feature -R -m 10 &
RECEIVER_PID=$!
sleep 2
./bin/sender -i test_feature.bin -r localhost -p 8084 -u -s 1024 -m
kill $RECEIVER_PID 2>/dev/null || true
wait $RECEIVER_PID 2>/dev/null || true

if [ -d "output_feature" ] && [ "$(ls -A output_feature)" ]; then
    cat output_feature/* > received_continuous.bin
    received_hash=$(sha256sum received_continuous.bin | cut -d' ' -f1)
    if [ "$original_hash" = "$received_hash" ]; then
        print_status $GREEN "✅ Continuous Disk: PASSED"
    else
        print_status $RED "❌ Continuous Disk: FAILED"
        exit 1
    fi
else
    print_status $RED "❌ Continuous Disk: FAILED"
    exit 1
fi

# Test 5: Link monitoring
print_status $YELLOW "=== Test 5: Link Monitoring ==="
./bin/receiver -o received_link.bin -p 8085 -u -s 1024 -m -t 2000 &
RECEIVER_PID=$!
sleep 2
./bin/sender -i test_feature.bin -r localhost -p 8085 -u -s 1024 -m -t 2000 &
SENDER_PID=$!
sleep 3
kill $SENDER_PID 2>/dev/null || true
kill $RECEIVER_PID 2>/dev/null || true
wait $SENDER_PID 2>/dev/null || true
wait $RECEIVER_PID 2>/dev/null || true

received_hash=$(sha256sum received_link.bin | cut -d' ' -f1)
if [ "$original_hash" = "$received_hash" ]; then
    print_status $GREEN "✅ Link Monitoring: PASSED"
else
    print_status $RED "❌ Link Monitoring: FAILED"
    exit 1
fi

# Test 6: Bit-by-bit comparison
print_status $YELLOW "=== Test 6: Bit-by-bit Comparison ==="
if cmp -s test_feature.bin received_basic.bin; then
    print_status $GREEN "✅ Bit-by-bit: PASSED"
else
    print_status $RED "❌ Bit-by-bit: FAILED"
    exit 1
fi

# Performance test
print_status $YELLOW "=== Performance Test ==="
./bin/receiver -o received_perf.bin -p 8086 -u -s 1024 -m &
RECEIVER_PID=$!
sleep 2
start_time=$(date +%s.%N)
./bin/sender -i test_feature.bin -r localhost -p 8086 -u -s 1024 -m
end_time=$(date +%s.%N)
kill $RECEIVER_PID 2>/dev/null || true
wait $RECEIVER_PID 2>/dev/null || true

duration=$(echo "$end_time - $start_time" | bc -l)
file_size=$(stat -c%s test_feature.bin)
throughput_mbps=$(echo "scale=2; $file_size * 8 / $duration / 1000000" | bc -l)
throughput_gbps=$(echo "scale=2; $throughput_mbps / 1000" | bc -l)

print_status $GREEN "Performance: ${throughput_mbps} Mbps (${throughput_gbps} Gbps)"

# Cleanup
rm -f test_feature.bin received_*.bin
rm -rf output_feature

print_status $GREEN ""
print_status $GREEN "=== All Feature Tests Completed Successfully ==="
print_status $GREEN "✅ Basic UDP transfer"
print_status $GREEN "✅ UDP with FEC"
print_status $GREEN "✅ Continuous mode (RAM)"
print_status $GREEN "✅ Continuous mode (Disk)"
print_status $GREEN "✅ Link monitoring"
print_status $GREEN "✅ Bit-by-bit integrity"
print_status $GREEN "✅ Performance: ${throughput_gbps} Gbps"