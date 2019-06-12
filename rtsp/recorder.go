package rtsp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/EasyDarwin/EasyDarwin/models"
	"github.com/EasyDarwin/EasyDarwin/record"
)

type _Recorder struct {
	taskExecute *models.TaskExecute
	pusher      Pusher
	queue       chan *RTPPack
	// hook and state
	onStops     []func()
	runningFlag bool
	inBytes     uint
	outBytes    uint
	startTime   time.Time
	// storage
	block       *models.Block
	blockBuffer *bytes.Buffer
}

// NewRecorder get data from pushers
func NewRecorder(task *models.Task, pusher Pusher) (Player, error) {
	taskExecute, err := models.ExecuteTask(task, pusher.SDPRaw())
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

const BlockHeaderLen = 20

var _20byteDummy = []byte{
	0x24, 0x24, 0x24, 0x24, // size of block
	0x00, 0x01, 0x02, 0x03, // reserved
	0x04, 0x05, 0x06, 0x07, // reserved
	0x08, 0x09, 0x0A, 0x0B, // reserved
	0x0C, 0x0D, 0x0E, 0x0F, // reserved
}

func (recorder *_Recorder) allocBlock() {
	recorder.block = record.NewBlock()
	recorder.block.StartTime = time.Now().Unix()

	// block bytes op
	recorder.blockBuffer = bytes.NewBuffer(recorder.block.Data)
	recorder.blockBuffer.Reset()
	// 20 bytes place holder
	recorder.blockBuffer.Write(_20byteDummy)
}

func (recorder *_Recorder) handleRTPPacket(pack *RTPPack) {
	if pack.SerializeToRecordLength()+recorder.blockBuffer.Len() <= record.BlockSize() {
		pack.SerializeToRecord(recorder.blockBuffer)
		return
	}

	log.WithFields(logrus.Fields{
		"cap": recorder.blockBuffer.Cap(),
		"len": recorder.blockBuffer.Len(),
	}).Debug("Block full")

	// fill block info
	recorder.block.EndTime = time.Now().Unix()
	recorder.block.Size = int32(recorder.blockBuffer.Len())
	// write block size in first 4 bytes
	binary.LittleEndian.PutUint32(recorder.block.Data, uint32(recorder.blockBuffer.Len()))

	if err := record.InsertBlock(recorder.taskExecute, recorder.block); nil != err {
		log.WithFields(logrus.Fields{
			"error": err,
			"ID":    recorder.ID(),
		}).Info("Recorder insert block")
	} else {
		recorder.outBytes += uint(record.BlockSize())
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
