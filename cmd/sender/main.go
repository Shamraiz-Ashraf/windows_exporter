package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"high-throughput-stream/pkg/stream"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func main() {
	// Parse command line flags
	var (
		configFile = flag.String("config", "", "Configuration file path")
		remoteAddr = flag.String("remote", "localhost", "Remote receiver address")
		port       = flag.Int("port", 8080, "Port number")
		inputFile  = flag.String("input", "", "Input file path")
		fileFormat = flag.String("format", "bin", "File format (bin or pcap)")
		enableFEC  = flag.Bool("fec", false, "Enable Forward Error Correction")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	// Load configuration
	config := loadConfig(*configFile)
	
	// Override config with command line flags
	if *remoteAddr != "localhost" {
		config.RemoteAddr = *remoteAddr
	}
	if *port != 8080 {
		config.Port = *port
	}
	if *inputFile != "" {
		config.InputFile = *inputFile
	}
	if *fileFormat != "bin" {
		config.FileFormat = *fileFormat
	} else {
		config.FileFormat = "bin"
	}
	if *enableFEC {
		config.EnableFEC = true
	}
	if *verbose {
		config.LogLevel = "debug"
	}

	// Validate required parameters
	if config.InputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: Input file is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Check if input file exists
	if _, err := os.Stat(config.InputFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Input file %s does not exist\n", config.InputFile)
		os.Exit(1)
	}

	// Setup logging
	setupLogging(config.LogLevel)

	logger := logrus.New()
	logger.Info("Starting high-throughput sender")
	logger.Infof("Configuration: %+v", config)

	// Create sender
	sender := stream.NewSender(config)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Connect to receiver
	logger.Info("Connecting to receiver...")
	if err := sender.Connect(); err != nil {
		logger.Fatalf("Failed to connect: %v", err)
	}

	// Start transfer in background
	transferDone := make(chan error, 1)
	go func() {
		err := sender.SendFile()
		transferDone <- err
	}()

	// Monitor transfer progress
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := sender.GetStats()
				throughputGbps := stream.ConvertToGbps(stats.Throughput)
				logger.Infof("Progress: %d bytes sent, %.2f Gbps, %d packets sent, %d retransmitted",
					stats.BytesSent, throughputGbps, stats.PacketsSent, stats.PacketsRetransmitted)
			case <-transferDone:
				return
			}
		}
	}()

	// Wait for completion or signal
	select {
	case err := <-transferDone:
		if err != nil {
			logger.Errorf("Transfer failed: %v", err)
			os.Exit(1)
		}
		logger.Info("Transfer completed successfully")
		
		// Print final statistics
		stats := sender.GetStats()
		duration := stats.EndTime.Sub(stats.StartTime)
		throughputGbps := stream.ConvertToGbps(stats.Throughput)
		
		fmt.Printf("\n=== Transfer Statistics ===\n")
		fmt.Printf("Duration: %v\n", duration)
		fmt.Printf("Bytes Sent: %d\n", stats.BytesSent)
		fmt.Printf("Packets Sent: %d\n", stats.PacketsSent)
		fmt.Printf("Packets Retransmitted: %d\n", stats.PacketsRetransmitted)
		fmt.Printf("Average Throughput: %.2f Gbps\n", throughputGbps)
		fmt.Printf("Errors: %d\n", stats.Errors)
		
		if stats.FECPacketsSent > 0 {
			fmt.Printf("FEC Packets Sent: %d\n", stats.FECPacketsSent)
		}

	case <-sigChan:
		logger.Info("Received shutdown signal, stopping transfer...")
		sender.Close()
	}

	logger.Info("Sender shutdown complete")
}

// loadConfig loads configuration from file or uses defaults
func loadConfig(configFile string) *stream.StreamConfig {
	config := &stream.StreamConfig{
		// Network configuration
		LocalAddr:  "0.0.0.0",
		RemoteAddr: "localhost",
		Port:       8080,

		// Performance configuration
		BufferSize: stream.DefaultBufferSize,
		PacketSize: 8192,
		WindowSize: stream.DefaultWindowSize,

		// FEC configuration
		EnableFEC:     false,
		FECRedundancy: 0.2,

		// Timing configuration
		Timeout:           30 * time.Second,
		RetryInterval:     100 * time.Millisecond,
		HeartbeatInterval: 1 * time.Second,

		// Logging
		LogLevel:      "info",
		EnableMetrics: true,
	}

	if configFile != "" {
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to read config file: %v\n", err)
		} else {
			// Override defaults with config file values
			if viper.IsSet("network.local_addr") {
				config.LocalAddr = viper.GetString("network.local_addr")
			}
			if viper.IsSet("network.remote_addr") {
				config.RemoteAddr = viper.GetString("network.remote_addr")
			}
			if viper.IsSet("network.port") {
				config.Port = viper.GetInt("network.port")
			}
			if viper.IsSet("performance.buffer_size") {
				config.BufferSize = viper.GetInt("performance.buffer_size")
			}
			if viper.IsSet("performance.packet_size") {
				config.PacketSize = viper.GetInt("performance.packet_size")
			}
			if viper.IsSet("performance.window_size") {
				config.WindowSize = viper.GetInt("performance.window_size")
			}
			if viper.IsSet("fec.enable") {
				config.EnableFEC = viper.GetBool("fec.enable")
			}
			if viper.IsSet("fec.redundancy") {
				config.FECRedundancy = viper.GetFloat64("fec.redundancy")
			}
			if viper.IsSet("timing.timeout") {
				config.Timeout = viper.GetDuration("timing.timeout")
			}
			if viper.IsSet("timing.retry_interval") {
				config.RetryInterval = viper.GetDuration("timing.retry_interval")
			}
			if viper.IsSet("timing.heartbeat_interval") {
				config.HeartbeatInterval = viper.GetDuration("timing.heartbeat_interval")
			}
			if viper.IsSet("logging.level") {
				config.LogLevel = viper.GetString("logging.level")
			}
			if viper.IsSet("logging.enable_metrics") {
				config.EnableMetrics = viper.GetBool("logging.enable_metrics")
			}
		}
	}

	return config
}

// setupLogging configures logging based on level
func setupLogging(level string) {
	switch level {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}