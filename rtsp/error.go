package rtsp

import "errors"

// Common errors
var (
	ErrorNeedMore     = errors.New("Need more data")
	ErrorChannelMap   = errors.New("Channel is not mapped")
	errorDecodeRTP    = errors.New("error when deocde RTP")
	ErrorRTPTooShort  = errors.New("RTP packet is too short")
	ErrorSDPMalformed = errors.New("SDP malformed")
	ErrorDB           = errors.New("DB error")
)
