package rtsp

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EasyDarwin/EasyDarwin/record"
	"github.com/pixelbender/go-sdp/sdp"
	"github.com/sirupsen/logrus"
)

type _VODState int

// PusherMode constants
const (
	// After NewVOD
	_VODStateInit _VODState = iota
	// After go vod.Start() is called, start read block
	_VODStateRun
	// After vod.Stop() is called
	_VODStateStop
)

// VOD impl rtsp.Pusher
type VOD struct {
	*defaultPusher
	_ID  string
	path string
	// state
	startBlock      *record.Block
	state           _VODState
	handleRTPPacket func(*RTPPack, *RTPInfo) (bool, error)
	// base duration calc
	baseStart    time.Time
	baseDuration time.Duration
	scale        int
	ticker       *time.Ticker
	// send duration calc
	sendDuration *RTPTimeDuration
	// info
	startAt time.Time
	// SDP
	_SDPRaw      string
	_SDP         *sdp.Session
	_VControl    string
	_VCodec      string
	_AChannelNum int
	_AControl    []string
	_ACodec      []string
	// message loop
	messageChannel chan vodCommand
	// data flow
	blockChannel chan *record.Block
	queue        chan *RTPPack
	// stop
	stopChannel   chan int
	stopWaitGroup *sync.WaitGroup
	StopHandles   []func()
}

type vodCommand interface {
	Do() error
}

// NewVOD returns
// startBlock needs ID, executeID and taskID
// TODO: when rtps.Session stop, stop VOD
func NewVOD(server *Server, ID string, path string, startBlock *record.Block) (_ *VOD, err error) {
	vod := &VOD{
		defaultPusher: newDefaultPusher(server),
		_ID:           ID,
		path:          path,

		scale: 0,

		startBlock: startBlock,
		state:      _VODStateInit,

		_AControl: []string{"not set up audio 01", "not set up audio 02"},
		_ACodec:   []string{"invalid codec", "invalid codec"},

		messageChannel: make(chan vodCommand, 4),
		stopChannel:    make(chan int, 1),
		stopWaitGroup:  &sync.WaitGroup{},

		blockChannel: make(chan *record.Block, 1),
		queue:        make(chan *RTPPack, config.Player.SendQueueLength),
	}
	// Get ready SDP
	vod._SDPRaw = startBlock.TaskExecute.SDPRaw
	vod._SDP, err = sdp.ParseString(startBlock.TaskExecute.SDPRaw)
	if nil != err {
		log.WithError(err).WithField("sdp", startBlock.TaskExecute.SDPRaw).Error("New VOD")
		return nil, ErrorSDPMalformed
	}

	for _, media := range vod._SDP.Media {
		switch media.Type {
		case "video":
			vod._VControl = media.Attributes.Get("control")
			vod._VCodec = media.Formats[0].Name
			log.Infof("video codec[%s]", vod._VCodec)
		case "audio":
			vod._AControl[vod._AChannelNum] = media.Attributes.Get("control")
			vod._ACodec[vod._AChannelNum] = media.Formats[0].Name
			log.Infof("audio codec[%s]", vod._ACodec[vod._AChannelNum])
			vod._AChannelNum++
		}
	}

	vod.AddOnStopHandle(vod.removeFromServer)

	return vod, nil
}

func (vod *VOD) removeFromServer() {
	vod.defaultPusher.server.RemovePusher(vod)
}

// ID of VOD for user control
func (vod *VOD) ID() string {
	return vod._ID
}

// Path of VOD for RTSP serve to ID
func (vod *VOD) Path() string {
	return vod.path
}

// Source of VOD
func (vod *VOD) Source() string {
	return fmt.Sprintf(
		"record://%s/%d",
		vod.startBlock.TaskExecute.TaskID,
		vod.startBlock.TaskExecute.ID,
	)
}

// TransType of VOD source
func (vod *VOD) TransType() string {
	return TRANS_TYPE_INTERNAL.String()
}

//StartAt of VOD
func (vod *VOD) StartAt() time.Time {
	return vod.startAt
}

// Mode of vod pusher
func (vod *VOD) Mode() PusherMode {
	return PusherModeVOD
}

// Start VOD action
func (vod *VOD) Start() {
	// Like the fist cmd
	vod.start()

	vod.state = _VODStateRun

	for cmd := range vod.messageChannel {
		cmd.Do()
	}
}

func (vod *VOD) resetTimer() {
	vod.baseStart = time.Now()
	// pre send 500 ms video data
	vod.baseDuration = 500 * time.Millisecond // TODO: config it!
	vod.sendDuration = nil
	vod.handleRTPPacket = vod.findFirstVideoRTPPacket
	vod.ticker = time.NewTicker(40 * time.Millisecond)
}

func (vod *VOD) start() {
	vod.resetTimer()

	vod.stopWaitGroup.Add(3)
	go vod.readBlockLoop()
	go vod.sendControlLoop()
	go vod.brocastLoop()
}

func (vod *VOD) readBlockLoop() {
	defer func() {
		close(vod.blockChannel)
		for _ = range vod.blockChannel {
			// speed up the close signal  to pick frame loop
		}
		vod.stopWaitGroup.Done()
		log.WithField("id", vod.ID()).Debug("vod.readBlockLoop exit")
	}()

	block := vod.startBlock
	blockInfo := *block
	for {
		// ReadBlockInfo
		{
			err := record.GetBlockByID(&blockInfo)
			if nil != err {
				log.WithFields(logrus.Fields{
					"ID":        block.ID,
					"executeID": block.TaskExecute.ID,
					"taskID":    block.TaskExecute.TaskID,
				}).Error("record.GetBlock")
				return
			}
			log.WithField("protobuf", blockInfo.String()).Debug("Read block info")
			block = record.NewBlock()
			record.AssignBlockButData(block, &blockInfo)
			// change state to next block
			blockInfo.ID++
			log.WithFields(logrus.Fields{
				"ID":        vod.ID(),
				"taskID":    block.TaskExecute.TaskID,
				"executeID": block.TaskExecute.ID,
				"blockID":   block.ID,
			}).Info("VOD read next block")
		}

		// ReadBlockData
		err := record.ReadBlockData(block)
		if nil != err {
			// TODO: tolarent error
			vod.Stop()
		}

		// Send block Data
		select {
		case <-vod.stopChannel:
			return
		case vod.blockChannel <- block:
			block = nil // hand over block
		}
	}
}

func makeupBlockData(block *record.Block) []byte {
	blockHeader := block.Data[:BlockHeaderLen]
	blockLen := binary.LittleEndian.Uint32(blockHeader)
	return block.Data[BlockHeaderLen:blockLen]
}

func (vod *VOD) nextRTPPacket(_data []byte) (data []byte, packet *RTPPack, info *RTPInfo, err error) {
	// TODO: limit search speed
	data = _data
	var l int

	for len(data) > 0 {
		// Derialize Packet From Data
		packet, l, err = DerializeFromRecord(data)
		if nil != err {
			log.WithError(err).Error("DerializeFromRecord")
			err = errorDecodeRTP
			return
		}
		data = data[l:]

		info = ParseRTP(packet.Buffer.Bytes())
		if nil == info {
			log.WithField("bytes", packet.Buffer.Bytes()[:RTP_FIXED_HEADER_LENGTH]).Warn("ParseRTP")
			continue
		}

		return
	}

	return nil, nil, nil, nil
}

func (vod *VOD) findFirstVideoRTPPacket(packet *RTPPack, info *RTPInfo) (bool, error) {
	if info.PayloadType < 96 { // Not video, TODO: more sure than experince
		return true, nil
	}
	// found first video rtp, go to next state
	vod.sendDuration = NewRTPTimeDuration(90000, info.Timestamp)
	vod.handleRTPPacket = vod.sendRTPPacketByTimestamp

	return vod.sendRTPPacketByTimestamp(packet, info)
}

func (vod *VOD) sendRTPPacketByTimestamp(packet *RTPPack, info *RTPInfo) (bool, error) {
	if info.PayloadType < 96 { // Not video, TODO: more sure than experince
		vod.QueueRTP(packet)
		return true, nil
	}
	// found first video rtp, go to next state

	sendDuraion := vod.sendDuration.Calc(info.Timestamp)
	baseDuraion := vod.baseDuration / time.Second
	if sendDuraion > baseDuraion {
		return false, nil
	}

	vod.QueueRTP(packet)
	return true, nil
}

func (vod *VOD) updateBaseDurationOrParameter() {
	now := <-vod.ticker.C
	vod.baseDuration = now.Sub(vod.baseStart)
	if vod.scale >= 0 {
		vod.baseDuration = (vod.baseDuration << uint(vod.scale))
	} else {
		vod.baseDuration = (vod.baseDuration >> uint(-vod.scale))
	}
}

func (vod *VOD) sendControlLoop() {
	defer func() {
		// exit send loop
		close(vod.queue)
		vod.stopWaitGroup.Done()
		log.WithField("id", vod.ID()).Debug("vod.sendControlLoop exit")
	}()

	var packet *RTPPack
	var info *RTPInfo
	var err error
	var handled bool
	// find first video RTP packet
	for block := range vod.blockChannel {
		data := makeupBlockData(block)
		for len(data) > 0 {
			data, packet, info, err = vod.nextRTPPacket(data)
			if nil != err {
				log.WithError(err).Error("VOD.nextRTPPacket")
				break
			}

			handled, err = vod.handleRTPPacket(packet, info)
			if nil != err {
				log.WithError(err).Error("VOD.handleRTPPacket")
				vod.Stop()
				return
			}
			if handled {
				// Next RTP packet
				continue
			}
			// usually after all packets has been sent according to timestamp
			handled = false
			for !handled {
				// the entry of change send parameter
				vod.updateBaseDurationOrParameter()
				handled, err = vod.handleRTPPacket(packet, info)
				if nil != err {
					log.WithError(err).Error("VOD.handleRTPPacket")
					vod.Stop()
					return
				}
			}

		}
	}
}

// brocastLoop to players
func (vod *VOD) brocastLoop() {
	defer func() {
		vod.stopWaitGroup.Done()
		log.WithField("id", vod.ID()).Debug("vod.brocastLoop exit")
	}()

	for packet := range vod.queue {
		vod.defaultPusher.BroadcastRTP(packet)
	}
}

// QueueRTP vod impl
func (vod *VOD) QueueRTP(pack *RTPPack) {
	select {
	case vod.queue <- pack:
	default:
		log.WithField("id", vod.ID()).Warn("pusher drop packet")
	}
}

type vodCommandStop struct{ *VOD }

func (cmd *vodCommandStop) Do() error {
	if cmd.VOD.state != _VODStateRun {
		log.WithField("state", cmd.VOD.state).Warn("Not usual state when VOD stop")
		return nil
	}

	// Stop message loop
	close(cmd.VOD.messageChannel)

	// Stop read and send loop
	select {
	case cmd.VOD.stopChannel <- 1:
	default:
		log.Warn("VOD.Stop may be called twice")
	}
	cmd.VOD.stopWaitGroup.Wait()
	log.WithField("id", cmd.VOD.ID()).Debug("All vod loop closed")

	cmd.VOD.state = _VODStateStop

	for _, h := range cmd.VOD.StopHandles {
		h()
	}

	return nil
}

// AddOnStopHandle of VOD
func (vod *VOD) AddOnStopHandle(handle func()) {
	vod.StopHandles = append(vod.StopHandles, handle)
}

// Stop vod
func (vod *VOD) Stop() {
	cmd := &vodCommandStop{VOD: vod}
	vod.messageChannel <- cmd
}

// AControl of VOD
func (vod *VOD) AControl() []string {
	return vod._AControl
}

// VControl of VOD
func (vod *VOD) VControl() string {
	return vod._VControl
}

// ACodec of VOD
func (vod *VOD) ACodec() []string {
	return vod._ACodec
}

// VCodec of VOD
func (vod *VOD) VCodec() string {
	return vod._VCodec
}

// SDPRaw of VOD
func (vod *VOD) SDPRaw() string {
	return vod._SDPRaw
}

// RemovePlayer when it is zero, stop VOD
func (vod *VOD) RemovePlayer(player Player) {
	vod.defaultPusher.playersLocker.Lock()
	vod.defaultPusher.players = vod.defaultPusher.players.Delete(player.ID())
	left := vod.defaultPusher.players.Len()
	vod.defaultPusher.playersLocker.Unlock()

	if 0 == left {
		vod.Stop()
	}
}

func (vod *VOD) stopIfNonePlayer() {
	if 0 == vod.GetPlayers().Len() {
		vod.Stop()
	}
}

func getVOD(server *Server, session *Session, path string, pusher Pusher) Pusher {
	if nil != pusher {
		return pusher
	}
	if nil == session {
		// VOD must bind the first session for lifecycle
		return pusher
	}
	// /vod/[taskID]/[executeID]/[startTime(in second)]/[VODID]
	// nonce to avoid get another VOD,
	//    client should take the respoonseiblity to make sure it is unqieu
	// check path wether to trigger vod
	parts := strings.Split(path, "/")
	if len(parts) >= 6 {
		parts = parts[:6]
	} else {
		return pusher
	}
	parts = parts[1:] // skip first empty string
	// Is vod?
	if strings.Compare(parts[0], "vod") != 0 {
		return pusher
	}
	// Enter vod router, if fail below, return nil
	// Get Exceute
	taskID := parts[1]
	executeID, err := strconv.ParseInt(parts[2], 10, 63)
	if nil != err {
		log.WithError(err).Error("VOD URL parse executeID")
		return nil
	}
	startTime, err := strconv.ParseInt(parts[3], 10, 63)
	if nil != err {
		log.WithError(err).Error("VOD URL parse startTime")
		return nil
	}
	VODID := parts[4]
	// Get start block ans execute info
	startBlock := record.NewEmptyBlock()
	startBlock.TaskExecute = &record.TaskExecute{}
	startBlock.TaskExecute.ID = executeID
	startBlock.TaskExecute.TaskID = taskID

	// NOTICE: gete block info first, it will cover execute info
	err = record.GetBlockByTime(startBlock, startTime)
	if nil != err {
		log.WithError(err).Error("VOD Get start block info")
		return nil
	}

	err = record.GetExecuteTask(startBlock.TaskExecute)
	if nil != err {
		log.WithError(err).Error("VOD Get task execute info")
		return nil
	}

	vod, err := NewVOD(server, VODID, path, startBlock)
	if nil != err {
		log.WithError(err).Error("VOD New")
		return nil
	}

	// IMPORTANT: Add vod to server, unlike the RTSP real pusher added in rtsp-session
	if server.AddPusher(vod, false) {
		session.StopHandles = append(session.StopHandles, vod.stopIfNonePlayer)
	} else {
		// Maybe there is a same name RTSP vod request at same time, return it
		if samePusher := server.GetPusher(path, nil); nil != samePusher {
			return samePusher
		} else {
			log.WithField("path", path).Warn("Add VOD to server's pusher pool fail")
			// So it maybe not the same path vod or just removed when you get it(unlucky)
			// TODO: or not, this is a question
			return pusher
		}
	}

	return vod
}

func initVOD() error {
	Instance.AddOnGetPusherHandle(getVOD)
	return nil
}
