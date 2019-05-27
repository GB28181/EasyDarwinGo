package rtsp

import (
	"bytes"
	"encoding/binary"
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
	SequenceNumber int
	Timestamp      int
	SSRC           int
	Payload        []byte
	PayloadOffset  int
}

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
		SequenceNumber: int(binary.BigEndian.Uint16(rtpBytes[2:])),
		Timestamp:      int(binary.BigEndian.Uint32(rtpBytes[4:])),
		SSRC:           int(binary.BigEndian.Uint32(rtpBytes[8:])),
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

// DerializeFromRecordToRTPOverTCP returns
// channelMap [RTPPack.Type<<1 + RTPPack.Channel]
func DerializeFromRecordToRTPOverTCP(buf []byte, channelMap []int) (*RTPPack, []byte, int, error) {
	p, consumed, err := DerializeFromRecordToRTPOverUDP(buf)
	if nil != err {
		return nil, nil, 0, err
	}
	sendChannel := channelMap[(int(p.Type)<<1)+p.Channel]
	if sendChannel < 0 {
		return nil, nil, 0, ErrorChannelMap
	}
	// modify raw
	buf[0] = 0x24
	buf[1] = byte(sendChannel)

	return p, buf[:consumed], consumed, nil
}

// DerializeFromRecordToRTPOverUDP returns
func DerializeFromRecordToRTPOverUDP(buf []byte) (*RTPPack, int, error) {
	if len(buf) < 4 {
		return nil, 0, ErrorNeedMore
	}
	p := &RTPPack{
		Type:    RTPType(buf[0]),
		Channel: int(buf[1]),
	}
	length := int(binary.BigEndian.Uint16(buf[2:]))
	if len(buf) < 4+length {
		return nil, 0, ErrorNeedMore
	}
	p.Buffer = bytes.NewBuffer(buf[4 : 4+length])

	return p, 4 + length, nil
}
