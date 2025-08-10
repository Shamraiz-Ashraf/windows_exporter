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
		
		// New UDP and link monitoring options
		useUDP              = flag.Bool("udp", false, "Use UDP instead of TCP")
		udpPayloadSize      = flag.Int("udp-payload", 1024, "UDP payload size in bytes")
		enableLinkMonitor   = flag.Bool("link-monitor", false, "Enable link interruption monitoring")
		linkTimeout         = flag.Duration("link-timeout", 5*time.Second, "Link timeout duration")
		linkMonitorInterval = flag.Duration("link-monitor-interval", 1*time.Second, "Link monitoring interval")
		
		// Continuous mode options
		continuousMode      = flag.Bool("continuous", false, "Run in continuous mode")
		dataSink            = flag.String("data-sink", "disk", "Data sink: disk, ram, or none")
		outputDirectory     = flag.String("output-dir", "./output", "Output directory for continuous mode")
		maxFileSize         = flag.Int64("max-file-size", 100*1024*1024, "Maximum file size before rotation (bytes)")
		enableFileRotation  = flag.Bool("file-rotation", false, "Enable file rotation in continuous mode")
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
	
	// New UDP and link monitoring options
	if *useUDP {
		config.UseUDP = true
		config.UDPPayloadSize = *udpPayloadSize
	}
	if *enableLinkMonitor {
		config.EnableLinkMonitoring = true
		config.LinkTimeout = *linkTimeout
		config.LinkMonitorInterval = *linkMonitorInterval
	}
	
	// Continuous mode options
	if *continuousMode {
		config.ContinuousMode = true
		config.DataSink = *dataSink
		config.OutputDirectory = *outputDirectory
		config.MaxFileSize = *maxFileSize
		config.EnableFileRotation = *enableFileRotation
	}

	// Validate required parameters
	if config.InputFile == "" && !config.ContinuousMode {
		fmt.Fprintf(os.Stderr, "Error: Input file is required (unless in continuous mode)\n")
		flag.Usage()
		os.Exit(1)
	}

	// Check if input file exists (only if not in continuous mode)
	if config.InputFile != "" && !config.ContinuousMode {
		if _, err := os.Stat(config.InputFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Input file %s does not exist\n", config.InputFile)
			os.Exit(1)
		}
	}

	// Setup logging
	setupLogging(config.LogLevel)

	logger := logrus.New()
	logger.Info("Starting high-throughput sender")
	logger.Infof("Configuration: %+v", config)

	// Create sender based on protocol
	var sender interface {
		Connect() error
		SendFile() error
		GetStats() *stream.StreamStats
		Close() error
	}
	
	if config.UseUDP {
		sender = stream.NewUDPSender(config)
		logger.Info("Using UDP sender")
	} else {
		sender = stream.NewSender(config)
		logger.Info("Using TCP sender")
	}

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
				logger.Infof("Progress: %d bytes sent, %.2f Gbps, %d packets sent, %d retransmitted, Link: %v",
					stats.BytesSent, throughputGbps, stats.PacketsSent, stats.PacketsRetransmitted, stats.LastLinkStatus)
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
		fmt.Printf("Link Interruptions: %d\n", stats.LinkInterruptions)
		fmt.Printf("Resync Count: %d\n", stats.ResyncCount)
		
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
		UseUDP:     false,

		// Performance configuration
		BufferSize:     stream.DefaultBufferSize,
		PacketSize:     8192,
		WindowSize:     stream.DefaultWindowSize,
		UDPPayloadSize: stream.DefaultUDPPayloadSize,

		// FEC configuration
		EnableFEC:     false,
		FECRedundancy: 0.2,

		// Timing configuration
		Timeout:           30 * time.Second,
		RetryInterval:     100 * time.Millisecond,
		HeartbeatInterval: 1 * time.Second,
		
		// Link monitoring configuration
		LinkMonitorInterval: 1 * time.Second,
		LinkTimeout:         5 * time.Second,
		EnableLinkMonitoring: false,
		
		// Continuous mode configuration
		ContinuousMode:      false,
		DataSink:            "disk",
		OutputDirectory:     "./output",
		MaxFileSize:         100 * 1024 * 1024, // 100MB
		EnableFileRotation:  false,

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
			if viper.IsSet("network.use_udp") {
				config.UseUDP = viper.GetBool("network.use_udp")
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
			if viper.IsSet("performance.udp_payload_size") {
				config.UDPPayloadSize = viper.GetInt("performance.udp_payload_size")
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
			if viper.IsSet("link_monitoring.enable") {
				config.EnableLinkMonitoring = viper.GetBool("link_monitoring.enable")
			}
			if viper.IsSet("link_monitoring.interval") {
				config.LinkMonitorInterval = viper.GetDuration("link_monitoring.interval")
			}
			if viper.IsSet("link_monitoring.timeout") {
				config.LinkTimeout = viper.GetDuration("link_monitoring.timeout")
			}
			if viper.IsSet("continuous_mode.enable") {
				config.ContinuousMode = viper.GetBool("continuous_mode.enable")
			}
			if viper.IsSet("continuous_mode.data_sink") {
				config.DataSink = viper.GetString("continuous_mode.data_sink")
			}
			if viper.IsSet("continuous_mode.output_directory") {
				config.OutputDirectory = viper.GetString("continuous_mode.output_directory")
			}
			if viper.IsSet("continuous_mode.max_file_size") {
				config.MaxFileSize = viper.GetInt64("continuous_mode.max_file_size")
			}
			if viper.IsSet("continuous_mode.enable_file_rotation") {
				config.EnableFileRotation = viper.GetBool("continuous_mode.enable_file_rotation")
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