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
)