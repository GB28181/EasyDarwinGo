package rtsp

import (
	"bytes"
	"fmt"
	"time"

	"github.com/ncw/directio"
)

type _Recorder struct {
	id     string
	pusher *Pusher
	queue  chan *RTPPack
	// hook
	onStops []func()
	// storage
	handleRTPPacket func(*RTPPack)
	raw             []byte
	buffer          *bytes.Buffer
	bufferStartTime time.Time
}

// NewRecorder get data from pushers
func NewRecorder(ID string, pusher *Pusher) Player {
	recorder := &_Recorder{
		id:     ID,
		pusher: pusher,
		queue:  make(chan *RTPPack, config.Record.ReceiveQueueLength),
	}
	recorder.handleRTPPacket = recorder.startBuffer
	recorder.AddOnStop(recorder.removeFromPusher)

	return recorder
}

func (recorder *_Recorder) removeFromPusher() {
	recorder.pusher.RemovePlayer(recorder)
}

func (recorder *_Recorder) AddOnStop(fn func()) {
	recorder.onStops = append(recorder.onStops, fn)
}

func (recorder *_Recorder) ID() string {
	return recorder.id
}

func (recorder *_Recorder) Path() string {
	return recorder.pusher.Path()
}

func (recorder *_Recorder) TransType() TransType {
	return TRANS_TYPE_INTERNAL
}

func (recorder *_Recorder) InBytes() int {
	// TODO:
	return 0
}

func (recorder *_Recorder) OutBytes() int {
	// TODO:
	return 0
}

func (recorder *_Recorder) StartAt() time.Time {
	// TODO:
	return time.Now()
}

func (recorder *_Recorder) QueueRTP(pack *RTPPack) Player {
	if pack == nil {
		fmt.Println("player queue enter nil pack, drop it")
		return recorder
	}
	select {
	case recorder.queue <- pack:
	default:
		fmt.Println("recorder queue full, drop it")
	}
	return recorder
}

func (recorder *_Recorder) startBuffer(pack *RTPPack) {
	recorder.raw = directio.AlignedBlock(2 * 1024 * 1024)
	recorder.buffer = bytes.NewBuffer(recorder.raw)

	pack.SerializeToRecord(recorder.buffer)

	recorder.handleRTPPacket = recorder.appendBuffer
}

func (recorder *_Recorder) appendBuffer(pack *RTPPack) {
	if pack.SerializeToRecordLength()+recorder.buffer.Len() <= 2*1024*1024 {
		pack.SerializeToRecord(recorder.buffer)
		return
	}
	// TODO: insert into
	recorder.handleRTPPacket = recorder.startBuffer
}

func (recorder *_Recorder) Start() {
	for pack := range recorder.queue {
		if pack == nil {
			continue
		}
		recorder.handleRTPPacket(pack)
	}
	fmt.Println("recorder send queue stopped, quit send loop")
}

func (recorder *_Recorder) Stop() {
	if nil == recorder.queue {
		return
	}
	close(recorder.queue)
	recorder.queue = nil
	// callback hook
	for _, onStop := range recorder.onStops {
		onStop()
	}
}
