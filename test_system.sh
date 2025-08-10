#!/bin/bash

# Test script for high-throughput stream system

echo "=== High-Throughput Stream System Test ==="

# Clean up any existing files
rm -f received.bin test_small.bin

# Create a small test file (1MB)
echo "Creating test file..."
dd if=/dev/urandom of=test_small.bin bs=1M count=1 2>/dev/null

echo "Test file created: $(ls -lh test_small.bin)"

# Start receiver in background
echo "Starting receiver..."
./bin/receiver -output=received.bin -port=8080 &
RECEIVER_PID=$!

# Wait for receiver to start
sleep 2

# Start sender
echo "Starting sender..."
./bin/sender -input=test_small.bin -remote=localhost -port=8080

# Wait for transfer to complete
sleep 5

# Stop receiver
echo "Stopping receiver..."
kill $RECEIVER_PID 2>/dev/null

# Check if files match
echo "Verifying transfer..."
if cmp -s test_small.bin received.bin; then
    echo "✅ SUCCESS: Files match perfectly!"
    echo "Original: $(ls -lh test_small.bin)"
    echo "Received: $(ls -lh received.bin)"
else
    echo "❌ FAILED: Files do not match!"
    echo "Original: $(ls -lh test_small.bin)"
    echo "Received: $(ls -lh received.bin)"
    exit 1
fi

# Clean up
rm -f test_small.bin received.bin

echo "=== Test completed successfully ==="