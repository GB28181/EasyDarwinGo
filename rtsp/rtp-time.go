package rtsp

import (
	"time"
)

// RTPTimeDuration calcs
type RTPTimeDuration struct {
	rate       uint64
	start      uint64
	accumulate uint64
}

// NewRTPTimeDuration returns
func NewRTPTimeDuration(rate uint64, start uint32) *RTPTimeDuration {
	return &RTPTimeDuration{
		rate:  rate,
		start: uint64(start),
	}
}

// RTPTimestampTop is 2^32
const RTPTimestampTop = uint64(1) << 32

// Calc duration to now, unit second
func (d *RTPTimeDuration) Calc(_now uint32) time.Duration {
	now := uint64(_now)
	// NOTICE: there is a bug when a RTP packet come late,
	// it is too complicated to detect here, please filter it when receive it.
	if (now - d.start) < 0 {
		// 0                                      2^32
		// |<---> now ---------------- start <--->|
		d.accumulate += (RTPTimestampTop - d.start) + (now - 0)
		d.start = now
	}

	return time.Duration((d.accumulate + (now - d.start)) / d.rate)
}
