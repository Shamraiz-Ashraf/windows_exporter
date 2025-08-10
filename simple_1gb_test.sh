#!/bin/bash

# Simple 1GB Test Script
echo "=== Simple 1GB Test ==="

# Check binaries
if [ ! -f "./bin/sender" ] || [ ! -f "./bin/receiver" ]; then
    echo "Error: Binaries not found"
    exit 1
fi

# Create 1GB test file
echo "Creating 1GB test file..."
dd if=/dev/urandom of=test_1gb.bin bs=1M count=1024 2>/dev/null
echo "1GB file created: $(ls -lh test_1gb.bin)"

# Calculate hash
original_hash=$(sha256sum test_1gb.bin | cut -d' ' -f1)
echo "Original hash: ${original_hash}"

# Start receiver
echo "Starting receiver..."
./bin/receiver -o received_1gb.bin -p 8081 -u -s 1024 -m &
RECEIVER_PID=$!

# Wait and start sender
sleep 3
echo "Starting sender..."
start_time=$(date +%s.%N)
./bin/sender -i test_1gb.bin -r localhost -p 8081 -u -s 1024 -m
end_time=$(date +%s.%N)

# Calculate throughput
duration=$(echo "$end_time - $start_time" | bc -l)
file_size=$(stat -c%s test_1gb.bin)
throughput_mbps=$(echo "scale=2; $file_size * 8 / $duration / 1000000" | bc -l)
throughput_gbps=$(echo "scale=2; $throughput_mbps / 1000" | bc -l)

echo "Transfer completed in ${duration}s"
echo "Throughput: ${throughput_mbps} Mbps (${throughput_gbps} Gbps)"

# Stop receiver
kill $RECEIVER_PID 2>/dev/null || true
wait $RECEIVER_PID 2>/dev/null || true

# Verify integrity
received_hash=$(sha256sum received_1gb.bin | cut -d' ' -f1)
if [ "$original_hash" = "$received_hash" ]; then
    echo "✅ SUCCESS: Files match perfectly!"
    echo "Original: $(ls -lh test_1gb.bin)"
    echo "Received: $(ls -lh received_1gb.bin)"
else
    echo "❌ FAILED: Files do not match!"
    echo "Original hash: ${original_hash}"
    echo "Received hash: ${received_hash}"
    exit 1
fi

# Bit-by-bit comparison
if cmp -s test_1gb.bin received_1gb.bin; then
    echo "✅ SUCCESS: Bit-by-bit comparison passed!"
else
    echo "❌ FAILED: Bit-by-bit comparison failed!"
    exit 1
fi

echo "🎉 1GB test completed successfully!"