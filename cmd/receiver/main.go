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
		localAddr  = flag.String("local", "0.0.0.0", "Local address to bind to")
		port       = flag.Int("port", 8080, "Port number")
		outputFile = flag.String("output", "", "Output file path")
		fileFormat = flag.String("format", "bin", "File format (bin or pcap)")
		enableFEC  = flag.Bool("fec", false, "Enable Forward Error Correction")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	// Load configuration
	config := loadConfig(*configFile)
	
	// Override config with command line flags
	if *localAddr != "0.0.0.0" {
		config.LocalAddr = *localAddr
	}
	if *port != 8080 {
		config.Port = *port
	}
	if *outputFile != "" {
		config.OutputFile = *outputFile
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
	if config.OutputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: Output file is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Setup logging
	setupLogging(config.LogLevel)

	logger := logrus.New()
	logger.Info("Starting high-throughput receiver")
	logger.Infof("Configuration: %+v", config)

	// Create receiver
	receiver := stream.NewReceiver(config)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start receiver
	logger.Info("Starting receiver...")
	if err := receiver.Start(); err != nil {
		logger.Fatalf("Failed to start receiver: %v", err)
	}

	// Start file reception in background
	receiveDone := make(chan error, 1)
	go func() {
		err := receiver.ReceiveFile()
		receiveDone <- err
	}()

	// Monitor reception progress
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := receiver.GetStats()
				throughputGbps := stream.ConvertToGbps(stats.Throughput)
				logger.Infof("Progress: %d bytes received, %.2f Gbps, %d packets received, %d lost",
					stats.BytesReceived, throughputGbps, stats.PacketsReceived, stats.PacketsLost)
			case <-receiveDone:
				return
			}
		}
	}()

	// Wait for completion or signal
	select {
	case err := <-receiveDone:
		if err != nil {
			logger.Errorf("Reception failed: %v", err)
			os.Exit(1)
		}
		logger.Info("File reception completed successfully")
		
		// Print final statistics
		stats := receiver.GetStats()
		duration := stats.EndTime.Sub(stats.StartTime)
		throughputGbps := stream.ConvertToGbps(stats.Throughput)
		
		fmt.Printf("\n=== Reception Statistics ===\n")
		fmt.Printf("Duration: %v\n", duration)
		fmt.Printf("Bytes Received: %d\n", stats.BytesReceived)
		fmt.Printf("Packets Received: %d\n", stats.PacketsReceived)
		fmt.Printf("Packets Lost: %d\n", stats.PacketsLost)
		fmt.Printf("Average Throughput: %.2f Gbps\n", throughputGbps)
		fmt.Printf("Errors: %d\n", stats.Errors)
		
		if stats.FECPacketsUsed > 0 {
			fmt.Printf("FEC Packets Used: %d\n", stats.FECPacketsUsed)
		}

	case <-sigChan:
		logger.Info("Received shutdown signal, stopping receiver...")
		receiver.Close()
	}

	logger.Info("Receiver shutdown complete")
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