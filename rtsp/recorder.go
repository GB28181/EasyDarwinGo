package rtsp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/EasyDarwin/EasyDarwin/rtsp/record"
)

type _Recorder struct {
	taskExecute *record.TaskExecute
	pusher      Pusher
	queue       chan *RTPPack
	// hook and state
	onStops     []func()
	runningFlag bool
	inBytes     uint
	outBytes    uint
	startTime   time.Time
	// storage
	block       *record.Block
	blockBuffer *bytes.Buffer
}

// NewRecorder get data from pushers
func NewRecorder(task *record.Task, pusher Pusher) (Player, error) {
	taskExecute, err := record.ExecuteTask(task, pusher.SDPRaw())
	if nil != err {
		return nil, err
	}

	recorder := &_Recorder{
		taskExecute: taskExecute,
		pusher:      pusher,
		queue:       make(chan *RTPPack, config.Record.ReceiveQueueLength),
		runningFlag: true,
		startTime:   time.Now(),
	}
	recorder.allocBlock()
	recorder.AddOnStop(recorder.removeFromPusher)

	return recorder, nil
}

func (recorder *_Recorder) removeFromPusher() {
	recorder.pusher.RemovePlayer(recorder)
}

func (recorder *_Recorder) AddOnStop(fn func()) {
	recorder.onStops = append(recorder.onStops, fn)
}

func (recorder *_Recorder) ID() string {
	return recorder.taskExecute.TaskID
}

func (recorder *_Recorder) Path() string {
	return recorder.pusher.Path()
}

func (recorder *_Recorder) TransType() TransType {
	return TRANS_TYPE_INTERNAL
}

func (recorder *_Recorder) InBytes() uint {
	return recorder.inBytes
}

func (recorder *_Recorder) OutBytes() uint {
	return recorder.outBytes
}

func (recorder *_Recorder) StartAt() time.Time {
	return recorder.startTime
}

func (recorder *_Recorder) QueueRTP(pack *RTPPack) Player {
	if pack == nil {
		fmt.Println("player queue enter nil pack, drop it")
		return recorder
	}
	select {
	case recorder.queue <- pack:
	default:
		if recorder.runningFlag {
			fmt.Println("recorder queue full, drop it")
		}
	}
	return recorder
}

var _4byteDummy = []byte{0x24, 0x24, 0x24, 0x24}

func (recorder *_Recorder) allocBlock() {
	recorder.block = record.NewBlock()
	recorder.block.StartTime = time.Now().Unix()

	// block bytes op
	recorder.blockBuffer = bytes.NewBuffer(recorder.block.Data)
	recorder.blockBuffer.Reset()
	recorder.blockBuffer.Write(_4byteDummy) // place holder for length
}

func (recorder *_Recorder) handleRTPPacket(pack *RTPPack) {
	if pack.SerializeToRecordLength()+recorder.blockBuffer.Len() <= config.Record.BlockSize {
		pack.SerializeToRecord(recorder.blockBuffer)
		return
	}

	recorder.block.EndTime = time.Now().Unix()

	log.WithFields(logrus.Fields{
		"cap": recorder.blockBuffer.Cap(),
		"len": recorder.blockBuffer.Len(),
	}).Debug("Block full")

	// write block size and insert to storage
	binary.LittleEndian.PutUint32(recorder.block.Data, uint32(recorder.blockBuffer.Len()))
	if err := recorder.taskExecute.InsertBlock(recorder.block); nil != err {
		log.WithFields(logrus.Fields{
			"error": err,
			"ID":    recorder.ID(),
		}).Info("Recorder insert block")
	}

	recorder.allocBlock()
	recorder.handleRTPPacket(pack)
}

func (recorder *_Recorder) Start() {
	var pack *RTPPack
	for recorder.runningFlag {
		pack = <-recorder.queue
		if pack == nil {
			continue
		}
		recorder.outBytes += uint(pack.Buffer.Len())
		recorder.handleRTPPacket(pack)
	}
	// send what's left in queue
LeftLoop:
	for {
		select {
		case pack = <-recorder.queue:
			if pack == nil {
				continue
			}
		default:
			break LeftLoop
		}
	}
	log.Infof("recorder[%s] quit send loop", recorder.ID())
}

func (recorder *_Recorder) Stop() {
	log.WithField("id", recorder.ID()).Info("recorder stop")
	recorder.runningFlag = false
	// callback hook
	for _, onStop := range recorder.onStops {
		onStop()
	}
}
