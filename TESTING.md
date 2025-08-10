# Testing Guide for C++ High-Throughput UDP Stream System

This guide explains how to run comprehensive tests for the C++ High-Throughput UDP Stream System, including 1GB file transfers, performance monitoring, and zero-loss verification.

## Prerequisites

1. **Build the system first:**
   ```bash
   make clean && make all
   ```

2. **Ensure sufficient disk space:**
   - At least 5GB free space for comprehensive tests
   - At least 3GB free space for 1GB transfer tests

3. **Required tools:**
   - `bc` (for calculations)
   - `sha256sum` (for integrity verification)
   - `dd` (for test file creation)
   - `cmp` (for bit-by-bit comparison)

## Test Scripts Overview

### 1. `simple_1gb_test.sh` - Basic 1GB Transfer Test
- **Purpose:** Simple 1GB file transfer with basic integrity verification
- **Duration:** ~5-10 minutes
- **Tests:** Basic UDP transfer, throughput measurement, SHA256 integrity check
- **Usage:**
  ```bash
  chmod +x simple_1gb_test.sh
  ./simple_1gb_test.sh
  ```

### 2. `feature_test.sh` - Comprehensive Feature Tests
- **Purpose:** Test all system features with smaller files
- **Duration:** ~2-5 minutes
- **Tests:**
  - Basic UDP transfer
  - UDP with FEC (Forward Error Correction)
  - Continuous mode with RAM data sink
  - Continuous mode with disk data sink and file rotation
  - Link monitoring and interruption handling
  - Bit-by-bit comparison
  - Performance measurement
- **Usage:**
  ```bash
  chmod +x feature_test.sh
  ./feature_test.sh
  ```

### 3. `performance_monitor.sh` - Performance Monitoring
- **Purpose:** Monitor system resources and throughput during transfers
- **Duration:** ~10-20 minutes
- **Tests:**
  - 100MB transfer with resource monitoring
  - 500MB transfer with resource monitoring
  - 1GB transfer with resource monitoring
  - Real-time CPU, memory, and network usage tracking
- **Usage:**
  ```bash
  chmod +x performance_monitor.sh
  ./performance_monitor.sh
  ```

### 4. `test_1gb_comprehensive.sh` - Comprehensive 1GB Test Suite
- **Purpose:** Full 1GB testing with all features and detailed analysis
- **Duration:** ~15-30 minutes
- **Tests:**
  - 1GB basic UDP transfer
  - 1GB UDP with FEC
  - 1GB continuous mode (RAM)
  - 1GB continuous mode (disk with rotation)
  - 1GB link interruption simulation
  - Bit-by-bit comparison
  - Performance analysis
- **Usage:**
  ```bash
  chmod +x test_1gb_comprehensive.sh
  ./test_1gb_comprehensive.sh
  ```

### 5. `run_all_tests.sh` - Complete Test Suite
- **Purpose:** Run all tests systematically with comprehensive reporting
- **Duration:** ~30-60 minutes
- **Tests:** All of the above plus additional verification
- **Usage:**
  ```bash
  chmod +x run_all_tests.sh
  ./run_all_tests.sh
  ```

## Quick Start Testing

For a quick verification that the system works:

```bash
# Make scripts executable
chmod +x *.sh

# Run basic feature tests (fastest)
./feature_test.sh

# Run 1GB transfer test
./simple_1gb_test.sh

# Run comprehensive test suite (recommended)
./run_all_tests.sh
```

## Expected Results

### Performance Targets
- **Target throughput:** 7+ Gbps
- **Zero-loss transmission:** 100% data integrity
- **Bit-perfect accuracy:** No data corruption

### Test Results Format
```
=== Test Results Summary ===
Feature Tests: PASSED
Performance Tests: PASSED
1GB Transfer: PASSED - 8.5 Gbps
Bit-by-bit Comparison: PASSED
Zero-loss Verification: PASSED
Throughput Target: ACHIEVED (8.5 Gbps >= 7.0 Gbps)
```

## Understanding Test Output

### Color Coding
- 🟢 **Green:** Success/PASSED
- 🔴 **Red:** Failure/FAILED
- 🟡 **Yellow:** Warning/Information
- 🔵 **Blue:** Progress/Status
- 🟣 **Purple:** Test sections
- 🔵 **Cyan:** Performance metrics

### Key Metrics
- **Throughput:** Measured in Mbps and Gbps
- **Transfer time:** Total time for file transfer
- **Integrity:** SHA256 hash comparison
- **Bit-perfect:** Binary comparison using `cmp`

## Troubleshooting

### Common Issues

1. **"Binaries not found"**
   ```bash
   make clean && make all
   ```

2. **"Insufficient disk space"**
   ```bash
   df -h  # Check available space
   # Free up space or use smaller test files
   ```

3. **"Permission denied"**
   ```bash
   chmod +x *.sh
   ```

4. **"Port already in use"**
   ```bash
   # Wait a few seconds between tests
   # Or kill any existing processes
   pkill -f "./bin/sender" 2>/dev/null || true
   pkill -f "./bin/receiver" 2>/dev/null || true
   ```

### Performance Issues

1. **Low throughput (< 7 Gbps)**
   - Check system resources (CPU, memory, network)
   - Ensure no other heavy processes running
   - Verify network interface speed

2. **Data corruption**
   - Check system stability
   - Verify sufficient memory
   - Check for disk errors

## Advanced Testing

### Custom File Sizes
Modify the test scripts to use different file sizes:

```bash
# In the test scripts, change:
dd if=/dev/urandom of=test_1gb.bin bs=1M count=1024
# To:
dd if=/dev/urandom of=test_500mb.bin bs=1M count=500
```

### Network Testing
For network-specific testing:

```bash
# Test with different UDP payload sizes
./bin/sender -i test.bin -r localhost -p 8080 -u -s 512  # 512-byte payloads
./bin/sender -i test.bin -r localhost -p 8080 -u -s 2048 # 2048-byte payloads
```

### Stress Testing
For stress testing, run multiple transfers simultaneously:

```bash
# Start multiple receivers
./bin/receiver -o received1.bin -p 8081 -u -s 1024 -m &
./bin/receiver -o received2.bin -p 8082 -u -s 1024 -m &
./bin/receiver -o received3.bin -p 8083 -u -s 1024 -m &

# Start multiple senders
./bin/sender -i test.bin -r localhost -p 8081 -u -s 1024 -m &
./bin/sender -i test.bin -r localhost -p 8082 -u -s 1024 -m &
./bin/sender -i test.bin -r localhost -p 8083 -u -s 1024 -m &
```

## Test Results Interpretation

### Success Criteria
- ✅ All tests pass
- ✅ Throughput ≥ 7 Gbps
- ✅ Zero data loss
- ✅ Bit-perfect transmission
- ✅ All features working

### Performance Analysis
- **Excellent:** ≥ 8 Gbps
- **Good:** 7-8 Gbps
- **Acceptable:** 6-7 Gbps
- **Needs improvement:** < 6 Gbps

### System Requirements for Optimal Performance
- **CPU:** Multi-core processor
- **Memory:** 4GB+ RAM
- **Network:** Gigabit Ethernet or faster
- **Storage:** SSD recommended for high throughput
- **OS:** Linux with optimized network stack

## Continuous Integration

For automated testing, the scripts can be integrated into CI/CD pipelines:

```bash
#!/bin/bash
# CI test script
set -e

make clean && make all
./run_all_tests.sh

# Check if all tests passed
if grep -q "FAILED" test_results.txt; then
    echo "Tests failed!"
    exit 1
else
    echo "All tests passed!"
    exit 0
fi
```

## Support

If you encounter issues with the tests:

1. Check the system logs for errors
2. Verify all dependencies are installed
3. Ensure sufficient system resources
4. Review the troubleshooting section above
5. Check the main README for system requirements