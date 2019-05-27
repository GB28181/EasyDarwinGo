package rtsp

import "errors"

// Common errors
var (
	ErrorNeedMore   = errors.New("Need more data")
	ErrorChannelMap = errors.New("Channel is not mapped")
)
