# Multi-stage build for high-throughput stream system
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binaries with optimizations
RUN make build

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S stream && \
    adduser -u 1001 -S stream -G stream

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/bin/sender /app/bin/sender
COPY --from=builder /app/bin/receiver /app/bin/receiver

# Copy configuration
COPY --from=builder /app/config.yaml /app/config.yaml

# Create data directory
RUN mkdir -p /app/data && chown -R stream:stream /app

# Switch to non-root user
USER stream

# Expose default port
EXPOSE 8080

# Set default command
CMD ["/app/bin/receiver", "-help"]