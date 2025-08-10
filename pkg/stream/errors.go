package stream

import "errors"

// Stream errors
var (
	ErrInvalidPacketSize = errors.New("invalid packet size")
	ErrInvalidMagic      = errors.New("invalid magic number")
	ErrPacketTooLarge    = errors.New("packet too large")
	ErrChecksumMismatch  = errors.New("checksum mismatch")
	ErrTimeout           = errors.New("operation timeout")
	ErrConnectionClosed  = errors.New("connection closed")
	ErrInvalidSequence   = errors.New("invalid sequence number")
	ErrBufferFull        = errors.New("buffer full")
	ErrFECDecodeFailed   = errors.New("FEC decode failed")
	ErrFileNotFound      = errors.New("file not found")
	ErrInvalidFileFormat = errors.New("invalid file format")
	ErrStreamClosed      = errors.New("stream closed")
	
	// Link monitoring errors
	ErrLinkInterrupted   = errors.New("link interrupted")
	ErrLinkTimeout       = errors.New("link timeout")
	ErrResyncFailed      = errors.New("resync failed")
	ErrInvalidUDPPayload = errors.New("invalid UDP payload size")
	
	// Continuous mode errors
	ErrInvalidDataSink   = errors.New("invalid data sink")
	ErrFileRotationFailed = errors.New("file rotation failed")
	ErrDirectoryNotFound = errors.New("output directory not found")
)