package rtsp

import (
	"bytes"
	"encoding/binary"
	"sync"
)

const (
	RTP_FIXED_HEADER_LENGTH = 12
)

// RTPPack of internal data switch
type RTPPack struct {
	Type    RTPType
	Buffer  *bytes.Buffer
	Channel int // Mostly audio channel index, can be video channel index
}

type RTPInfo struct {
	Version        int
	Padding        bool
	Extension      bool
	CSRCCnt        int
	Marker         bool
	PayloadType    int
	SequenceNumber uint16
	Timestamp      uint32
	SSRC           uint32
	Payload        []byte
	PayloadOffset  int
}

// RTPPack pool
var _RTPPackPool = &sync.Pool{
	New: newRTPPack,
}

func newRTPPack() interface{} {
	return &RTPPack{}
}

// NewRTPPack returns
func NewRTPPack() *RTPPack {
	return _RTPPackPool.Get().(*RTPPack)
}

// RecycleRTPPack to use
func RecycleRTPPack(packet *RTPPack) {
	_RTPPackPool.Put(packet)
}

// GetRTPTimestamp and nothing more
func GetRTPTimestamp(rtpBytes []byte) (uint32, error) {
	if len(rtpBytes) < RTP_FIXED_HEADER_LENGTH {
		return 0, ErrorRTPTooShort
	}
	return binary.BigEndian.Uint32(rtpBytes[4:]), nil
}

// ParseRTP to get info in head
func ParseRTP(rtpBytes []byte) *RTPInfo {
	if len(rtpBytes) < RTP_FIXED_HEADER_LENGTH {
		return nil
	}
	firstByte := rtpBytes[0]
	secondByte := rtpBytes[1]
	info := &RTPInfo{
		Version:   int(firstByte >> 6),
		Padding:   (firstByte>>5)&1 == 1,
		Extension: (firstByte>>4)&1 == 1,
		CSRCCnt:   int(firstByte & 0x0f),

		Marker:         secondByte>>7 == 1,
		PayloadType:    int(secondByte & 0x7f),
		SequenceNumber: binary.BigEndian.Uint16(rtpBytes[2:]),
		Timestamp:      binary.BigEndian.Uint32(rtpBytes[4:]),
		SSRC:           binary.BigEndian.Uint32(rtpBytes[8:]),
	}
	offset := RTP_FIXED_HEADER_LENGTH
	end := len(rtpBytes)
	if end-offset >= 4*info.CSRCCnt {
		offset += 4 * info.CSRCCnt
	}
	if info.Extension && end-offset >= 4 {
		extLen := 4 * int(binary.BigEndian.Uint16(rtpBytes[offset+2:]))
		offset += 4
		if end-offset >= extLen {
			offset += extLen
		}
	}
	if info.Padding && end-offset > 0 {
		paddingLen := int(rtpBytes[end-1])
		if end-offset >= paddingLen {
			end -= paddingLen
		}
	}
	info.Payload = rtpBytes[offset:end]
	info.PayloadOffset = offset
	if end-offset < 1 {
		return nil
	}

	return info
}

func (p *RTPPack) SerializeToRecordLength() int {
	return 4 + p.Buffer.Len()
}

// SerializeToRecord for storage
// return grow length
func (p *RTPPack) SerializeToRecord(buf *bytes.Buffer) {
	header := make([]byte, 4)
	// place holder for the magic number $
	header[0] = byte(p.Type)
	// change to actual channel when vod send
	header[1] = byte(p.Channel)
	// rest is same as RTP over TCP
	binary.BigEndian.PutUint16(header[2:], uint16(p.Buffer.Len()))

	buf.Write(header)
	buf.Write(p.Buffer.Bytes())
}

// DerializeFromRecord returns
func DerializeFromRecord(buf []byte) (*RTPPack, int, error) {
	if len(buf) < 4 {
		return nil, 0, ErrorNeedMore
	}
	p := NewRTPPack()
	p.Type = RTPType(buf[0])
	p.Channel = int(buf[1])

	length := int(binary.BigEndian.Uint16(buf[2:]))
	if len(buf) < 4+length {
		return nil, 0, ErrorNeedMore
	}
	p.Buffer = bytes.NewBuffer(buf[4 : 4+length])

	return p, 4 + length, nil
}
