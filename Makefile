# High-Throughput Stream Makefile

# Build configuration
BINARY_DIR = bin
SENDER_BINARY = $(BINARY_DIR)/sender
RECEIVER_BINARY = $(BINARY_DIR)/receiver

# Go configuration
GO = go
GOFLAGS = -ldflags="-s -w" # Strip debug info for smaller binaries

# Build targets
.PHONY: all build clean sender receiver test help

all: build

build: $(BINARY_DIR) sender receiver

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

sender: $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $(SENDER_BINARY) ./cmd/sender

receiver: $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $(RECEIVER_BINARY) ./cmd/receiver

# Development targets
dev-sender:
	$(GO) run ./cmd/sender -input=test.bin -remote=localhost -port=8080 -verbose

dev-receiver:
	$(GO) run ./cmd/receiver -output=received.bin -local=0.0.0.0 -port=8080 -verbose

# Test targets
test:
	$(GO) test -v ./pkg/stream/...

test-race:
	$(GO) test -race -v ./pkg/stream/...

test-bench:
	$(GO) test -bench=. -benchmem ./pkg/stream/...

# Performance testing
perf-test: build
	@echo "Starting performance test..."
	@echo "1. Start receiver in background..."
	@$(RECEIVER_BINARY) -output=perf_test_output.bin -port=8081 &
	@sleep 2
	@echo "2. Start sender..."
	@$(SENDER_BINARY) -input=test.bin -remote=localhost -port=8081
	@echo "3. Cleanup..."
	@pkill -f receiver || true
	@rm -f perf_test_output.bin

# Generate test data
generate-test-data:
	@echo "Generating 100MB test file..."
	@dd if=/dev/urandom of=test.bin bs=1M count=100 2>/dev/null || \
		$(GO) run -e 'package main; import "crypto/rand"; import "os"; b := make([]byte, 100*1024*1024); rand.Read(b); os.WriteFile("test.bin", b, 0644)' 2>/dev/null || \
		echo "Please install dd or create test.bin manually"

# Clean targets
clean:
	rm -rf $(BINARY_DIR)
	rm -f test.bin received.bin perf_test_output.bin

# Install dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Lint and format
lint:
	golangci-lint run

format:
	$(GO) fmt ./...
	$(GO) vet ./...

# Docker targets
docker-build:
	docker build -t high-throughput-stream .

docker-run-sender:
	docker run --network host high-throughput-stream sender -input=test.bin -remote=localhost -port=8080

docker-run-receiver:
	docker run --network host high-throughput-stream receiver -output=received.bin -local=0.0.0.0 -port=8080

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build sender and receiver binaries"
	@echo "  sender         - Build sender binary only"
	@echo "  receiver       - Build receiver binary only"
	@echo "  dev-sender     - Run sender in development mode"
	@echo "  dev-receiver   - Run receiver in development mode"
	@echo "  test           - Run tests"
	@echo "  test-race      - Run tests with race detection"
	@echo "  test-bench     - Run benchmarks"
	@echo "  perf-test      - Run performance test"
	@echo "  generate-test-data - Generate test data file"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Install dependencies"
	@echo "  lint           - Run linter"
	@echo "  format         - Format code"
	@echo "  docker-build   - Build Docker image"
	@echo "  help           - Show this help"

# Example usage
example:
	@echo "Example usage:"
	@echo ""
	@echo "1. Start receiver:"
	@echo "   ./bin/receiver -output=received.bin -port=8080"
	@echo ""
	@echo "2. Start sender:"
	@echo "   ./bin/sender -input=test.bin -remote=localhost -port=8080"
	@echo ""
	@echo "3. With FEC enabled:"
	@echo "   ./bin/sender -input=test.bin -remote=localhost -port=8080 -fec"
	@echo "   ./bin/receiver -output=received.bin -port=8080 -fec"
	@echo ""
	@echo "4. With custom configuration:"
	@echo "   ./bin/sender -config=config.yaml -input=test.bin"
	@echo "   ./bin/receiver -config=config.yaml -output=received.bin"