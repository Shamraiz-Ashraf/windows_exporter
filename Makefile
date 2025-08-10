# Makefile for High-Throughput UDP Stream System

# Compiler and flags
CXX = g++
CXXFLAGS = -std=c++17 -Wall -Wextra -O3 -march=native -mtune=native
CXXFLAGS += -DNDEBUG -fomit-frame-pointer
CXXFLAGS += -pthread

# Debug flags (uncomment for debugging)
# CXXFLAGS = -std=c++17 -Wall -Wextra -g -O0 -DDEBUG -pthread

# Directories
SRCDIR = src
BINDIR = bin
OBJDIR = obj

# Source files
SOURCES = $(wildcard $(SRCDIR)/*.cpp)
OBJECTS = $(SOURCES:$(SRCDIR)/%.cpp=$(OBJDIR)/%.o)

# Executables
SENDER = $(BINDIR)/sender
RECEIVER = $(BINDIR)/receiver

# Default target
all: $(SENDER) $(RECEIVER)

# Create directories
$(BINDIR):
	mkdir -p $(BINDIR)

$(OBJDIR):
	mkdir -p $(OBJDIR)

# Build sender
$(SENDER): $(OBJDIR)/main_sender.o $(OBJDIR)/udp_sender.o $(OBJDIR)/utils.o
	$(CXX) $(CXXFLAGS) -o $@ $^

# Build receiver
$(RECEIVER): $(OBJDIR)/main_receiver.o $(OBJDIR)/udp_receiver.o $(OBJDIR)/utils.o
	$(CXX) $(CXXFLAGS) -o $@ $^

# Compile source files
$(OBJDIR)/%.o: $(SRCDIR)/%.cpp | $(OBJDIR)
	$(CXX) $(CXXFLAGS) -c -o $@ $<

# Clean build artifacts
clean:
	rm -rf $(OBJDIR) $(BINDIR)

# Install dependencies (for Ubuntu/Debian)
install-deps:
	sudo apt-get update
	sudo apt-get install -y build-essential g++ make

# Test targets
test: $(SENDER) $(RECEIVER)
	@echo "=== Running C++ UDP Stream System Tests ==="
	@chmod +x test_cpp_system.sh
	@./test_cpp_system.sh

# Performance test
perf-test: $(SENDER) $(RECEIVER)
	@echo "=== Running Performance Tests ==="
	@chmod +x test_performance.sh
	@./test_performance.sh

# Benchmark
benchmark: $(SENDER) $(RECEIVER)
	@echo "=== Running Benchmarks ==="
	@chmod +x benchmark.sh
	@./benchmark.sh

# Create test data
test-data:
	@echo "Creating test data..."
	@dd if=/dev/urandom of=test_data.bin bs=1M count=10 2>/dev/null
	@echo "Test data created: test_data.bin (10MB)"

# Run with different configurations
test-basic: $(SENDER) $(RECEIVER)
	@echo "=== Basic UDP Transfer Test ==="
	@./test_cpp_system.sh basic

test-fec: $(SENDER) $(RECEIVER)
	@echo "=== UDP Transfer with FEC Test ==="
	@./test_cpp_system.sh fec

test-continuous: $(SENDER) $(RECEIVER)
	@echo "=== Continuous Mode Test ==="
	@./test_cpp_system.sh continuous

test-link-monitor: $(SENDER) $(RECEIVER)
	@echo "=== Link Monitoring Test ==="
	@./test_cpp_system.sh link-monitor

# Documentation
docs:
	@echo "=== Generating Documentation ==="
	@mkdir -p docs
	@echo "# High-Throughput UDP Stream System" > docs/README.md
	@echo "" >> docs/README.md
	@echo "## Features" >> docs/README.md
	@echo "- 1024-byte UDP payload transfer" >> docs/README.md
	@echo "- Bit-perfect transmission" >> docs/README.md
	@echo "- Link interruption awareness and resync" >> docs/README.md
	@echo "- Continuous mode with multiple data sinks" >> docs/README.md
	@echo "- Forward Error Correction (FEC)" >> docs/README.md
	@echo "- High-performance optimized for 7+ Gbps" >> docs/README.md

# Package for distribution
package: clean all docs
	@echo "=== Creating Package ==="
	@mkdir -p package
	@cp $(SENDER) $(RECEIVER) package/
	@cp -r docs package/
	@cp test_cpp_system.sh package/
	@cp README.md package/
	@tar -czf high-throughput-udp-stream.tar.gz package/
	@rm -rf package
	@echo "Package created: high-throughput-udp-stream.tar.gz"

# Development targets
dev: CXXFLAGS += -g -O0 -DDEBUG
dev: clean all

# Release build
release: CXXFLAGS += -DNDEBUG -O3 -march=native
release: clean all

# Help
help:
	@echo "Available targets:"
	@echo "  all          - Build sender and receiver (default)"
	@echo "  clean        - Remove build artifacts"
	@echo "  install-deps - Install system dependencies"
	@echo "  test         - Run all tests"
	@echo "  perf-test    - Run performance tests"
	@echo "  benchmark    - Run benchmarks"
	@echo "  test-data    - Create test data files"
	@echo "  test-basic   - Run basic UDP transfer test"
	@echo "  test-fec     - Run UDP transfer with FEC test"
	@echo "  test-continuous - Run continuous mode test"
	@echo "  test-link-monitor - Run link monitoring test"
	@echo "  docs         - Generate documentation"
	@echo "  package      - Create distribution package"
	@echo "  dev          - Development build with debug info"
	@echo "  release      - Optimized release build"
	@echo "  help         - Show this help message"

.PHONY: all clean install-deps test perf-test benchmark test-data test-basic test-fec test-continuous test-link-monitor docs package dev release help