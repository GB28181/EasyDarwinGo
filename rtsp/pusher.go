package rtsp

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/EasyDarwin/EasyDarwin/utils"
	"github.com/benbjohnson/immutable"
)

// PusherMode pull from server or push from client
type PusherMode int

// PusherMode constants
const (
	PusherModePush PusherMode = iota
	PusherModePull
	PusherModeVOD
)

// Pusher of RTSP server
type Pusher interface {
	// Info
	// ID must always be valid andd not changed
	ID() string
	Path() string
	Source() string
	TransType() string
	InBytes() uint
	OutBytes() uint
	StartAt() time.Time
	Mode() PusherMode
	// State
	Start()
	Stop()
	Server() *Server
	AddOnStopHandle(func())
	// Player
	AddPlayer(Player) error
	RemovePlayer(Player)
	ClearPlayer()
	GetPlayers() *immutable.Map
	GetPlayer(ID string) Player
	// Media
	QueueRTP(*RTPPack)
	AControl() []string
	VControl() string
	ACodec() []string
	VCodec() string
	SDPRaw() string
}

type _Pusher struct {
	*Session
	*RTSPClient
	players        *immutable.Map //SessionID <-> Player
	playersLocker  *utils.SpinLock
	gopCacheEnable bool
	gopCache       []*RTPPack
	gopCacheLock   sync.RWMutex

	spsppsInSTAPaPack bool
	queue             chan *RTPPack
}

func (pusher *_Pusher) String() string {
	if pusher.Session != nil {
		return pusher.Session.String()
	}
	return pusher.RTSPClient.String()
}

func (pusher *_Pusher) Mode() PusherMode {
	if pusher.Session != nil {
		return PusherModePush
	} else if pusher.RTSPClient != nil {
		return PusherModePull
	}
	log.Fatal("Unsupprted mode")
	return PusherModePush
}

func (pusher *_Pusher) Server() *Server {
	if pusher.Session != nil {
		return pusher.Session.Server
	}
	return pusher.RTSPClient.Server
}

func (pusher *_Pusher) SDPRaw() string {
	if pusher.Session != nil {
		return pusher.Session.SDPRaw
	}
	return pusher.RTSPClient.SDPRaw
}

func (pusher *_Pusher) Stoped() bool {
	if pusher.Session != nil {
		return pusher.Session.Stoped
	}
	return pusher.RTSPClient.Stoped
}

func (pusher *_Pusher) Path() string {
	if pusher.Session != nil {
		return pusher.Session.Path
	}
	if pusher.RTSPClient.CustomPath != "" {
		return pusher.RTSPClient.CustomPath
	}
	return pusher.RTSPClient.Path
}

func (pusher *_Pusher) ID() string {
	if pusher.Session != nil {
		return pusher.Session.ID
	}
	return pusher.RTSPClient.ID
}

func (pusher *_Pusher) VCodec() string {
	if pusher.Session != nil {
		return pusher.Session.VCodec
	}
	return pusher.RTSPClient.VCodec
}

func (pusher *_Pusher) ACodec() []string {
	if pusher.Session != nil {
		return pusher.Session.ACodec
	}
	return pusher.RTSPClient.ACodec
}

func (pusher *_Pusher) AControl() []string {
	if pusher.Session != nil {
		fmt.Printf("AControl return:%v", pusher.Session.AControl)
		return pusher.Session.AControl
	}
	fmt.Printf("AControl return:%v", pusher.RTSPClient.AControl)
	return pusher.RTSPClient.AControl
}

func (pusher *_Pusher) VControl() string {
	if pusher.Session != nil {
		return pusher.Session.VControl
	}
	return pusher.RTSPClient.VControl
}

func (pusher *_Pusher) Source() string {
	if pusher.Session != nil {
		return pusher.Session.URL
	}
	return pusher.RTSPClient.URL
}

func (pusher *_Pusher) AddOutputBytes(size int) {
	if pusher.Session != nil {
		pusher.Session.OutBytes += uint(size)
		return
	}
	pusher.RTSPClient.OutBytes += uint(size)
}

func (pusher *_Pusher) InBytes() uint {
	if pusher.Session != nil {
		return pusher.Session.InBytes
	}
	return pusher.RTSPClient.InBytes
}

func (pusher *_Pusher) OutBytes() uint {
	if pusher.Session != nil {
		return pusher.Session.OutBytes
	}
	return pusher.RTSPClient.OutBytes
}

func (pusher *_Pusher) TransType() string {
	if pusher.Session != nil {
		return pusher.Session.TransType.String()
	}
	return pusher.RTSPClient.TransType.String()
}

func (pusher *_Pusher) StartAt() time.Time {
	if pusher.Session != nil {
		return pusher.Session.StartAt
	}
	return pusher.RTSPClient.StartAt
}

func (pusher *_Pusher) AddOnStopHandle(handle func()) {
	if nil != pusher.RTSPClient {
		pusher.RTSPClient.StopHandles = append(pusher.RTSPClient.StopHandles, handle)
	} else if nil != pusher.Session {
		pusher.Session.StopHandles = append(pusher.Session.StopHandles, handle)
	}
}

// NewClientPusher returns
func NewClientPusher(client *RTSPClient) Pusher {
	pusher := &_Pusher{
		RTSPClient:     client,
		Session:        nil,
		players:        immutable.NewMap(nil),
		playersLocker:  &utils.SpinLock{},
		gopCacheEnable: config.RTSP.GopCacheEnable != 0,
		gopCache:       make([]*RTPPack, 0),

		queue: make(chan *RTPPack, config.Player.SendQueueLength),
	}
	client.RTPHandles = append(client.RTPHandles, pusher.QueueRTP)
	pusher.AddOnStopHandle(func() {
		pusher.ClearPlayer()
		pusher.Server().RemovePusher(pusher.ID())
	})

	return pusher
}

// NewPusher of session
func NewPusher(session *Session) Pusher {
	pusher := &_Pusher{
		Session:        session,
		RTSPClient:     nil,
		players:        immutable.NewMap(nil),
		playersLocker:  &utils.SpinLock{},
		gopCacheEnable: config.RTSP.GopCacheEnable != 0,
		gopCache:       make([]*RTPPack, 0),

		queue: make(chan *RTPPack, config.Player.SendQueueLength),
	}
	session.RTPHandles = append(session.RTPHandles, pusher.QueueRTP)
	pusher.AddOnStopHandle(func() {
		pusher.ClearPlayer()
		pusher.Server().RemovePusher(pusher.ID())
	})

	return pusher
}

func (pusher *_Pusher) QueueRTP(pack *RTPPack) {
	select {
	case pusher.queue <- pack:
	default:
		log.WithField("id", pusher.ID()).Warn("pusher drop packet")
	}
}

func (pusher *_Pusher) Start() {
	for !pusher.Stoped() {
		pack := <-pusher.queue
		if pack == nil {
			if !pusher.Stoped() {
				log.Error("_Pusher not stoped, but queue take out nil pack")
			}
			continue
		}

		if pusher.gopCacheEnable && pack.Type == RTP_TYPE_VIDEO {
			pusher.gopCacheLock.Lock()
			if rtp := ParseRTP(pack.Buffer.Bytes()); rtp != nil && pusher.shouldSequenceStart(rtp) {
				pusher.gopCache = make([]*RTPPack, 0)
			}
			pusher.gopCache = append(pusher.gopCache, pack)
			pusher.gopCacheLock.Unlock()
		}
		pusher.BroadcastRTP(pack)
	}
}

func (pusher *_Pusher) Stop() {
	if pusher.Session != nil {
		pusher.Session.Stop()
		return
	}
	pusher.RTSPClient.Stop()
}

func (pusher *_Pusher) BroadcastRTP(pack *RTPPack) *_Pusher {
	players := pusher.GetPlayers()

	for itPlayer := players.Iterator(); !itPlayer.Done(); {
		_, _player := itPlayer.Next()
		player := _player.(Player)
		player.QueueRTP(pack)
		pusher.AddOutputBytes(pack.Buffer.Len())
	}

	return pusher
}

func (pusher *_Pusher) GetPlayers() *immutable.Map {
	pusher.playersLocker.Lock()
	players := pusher.players
	pusher.playersLocker.Unlock()

	return players
}

func (pusher *_Pusher) GetPlayer(ID string) Player {
	_player, ok := pusher.players.Get(ID)
	if !ok {
		return nil
	}

	return _player.(Player)
}

func (pusher *_Pusher) AddPlayer(player Player) error {
	if pusher.gopCacheEnable {
		pusher.gopCacheLock.RLock()
		for _, pack := range pusher.gopCache {
			player.QueueRTP(pack)
			pusher.AddOutputBytes(pack.Buffer.Len())
		}
		pusher.gopCacheLock.RUnlock()
	}

	var playerIDExist bool
	pusher.playersLocker.Lock()
	if _, playerIDExist = pusher.players.Get(player.ID()); !playerIDExist {
		pusher.players = pusher.players.Set(player.ID(), player)
	}
	pusher.playersLocker.Unlock()

	if playerIDExist {
		return fmt.Errorf("Player[%s] already registed", player.ID())
	}

	go player.Start()

	return nil
}

// RemovePlayer from pusher, stop receive data
func (pusher *_Pusher) RemovePlayer(player Player) {
	pusher.playersLocker.Lock()
	pusher.players = pusher.players.Delete(player.ID())
	pusher.playersLocker.Unlock()

	log.Infof("player %s end, now player size[%d]\n", player.ID(), pusher.players.Len())
}

func (pusher *_Pusher) ClearPlayer() {
	pusher.playersLocker.Lock()
	oldPlayers := pusher.players
	pusher.players = immutable.NewMap(nil)
	pusher.playersLocker.Unlock()

	go func() {
		for itPlayer := oldPlayers.Iterator(); !itPlayer.Done(); {
			_, _player := itPlayer.Next()
			_player.(Player).Stop()
		}
	}()
}

func (pusher *_Pusher) shouldSequenceStart(rtp *RTPInfo) bool {
	if strings.EqualFold(pusher.VCodec(), "h264") {
		var realNALU uint8
		payloadHeader := rtp.Payload[0] //https://tools.ietf.org/html/rfc6184#section-5.2
		NaluType := uint8(payloadHeader & 0x1F)
		switch {
		case NaluType <= 23:
			realNALU = rtp.Payload[0]
			// log.Printf("Single NAL:%d", NaluType)
		case NaluType == 28 || NaluType == 29:
			realNALU = rtp.Payload[1]
			if realNALU&0x40 != 0 {
				// log.Printf("FU NAL End :%02X", realNALU)
			}
			if realNALU&0x80 != 0 {
				// log.Printf("FU NAL Begin :%02X", realNALU)
			} else {
				return false
			}
		case NaluType == 24:
			// log.Printf("STAP-A")
			off := 1
			singleSPSPPS := 0
			for {
				nalSize := ((uint16(rtp.Payload[off])) << 8) | uint16(rtp.Payload[off+1])
				if nalSize < 1 {
					return false
				}
				off += 2
				nalUnit := rtp.Payload[off : off+int(nalSize)]
				off += int(nalSize)
				realNALU = nalUnit[0]
				singleSPSPPS += int(realNALU & 0x1F)
				if off >= len(rtp.Payload) {
					break
				}
			}
			if singleSPSPPS == 0x0F {
				pusher.spsppsInSTAPaPack = true
				return true
			}
		}
		if realNALU&0x1F == 0x05 {
			if pusher.spsppsInSTAPaPack {
				return false
			}
			return true
		}
		if realNALU&0x1F == 0x07 { // maybe sps pps header + key frame?
			if len(rtp.Payload) < 200 { // consider sps pps header only.
				return true
			}
			return true
		}
		return false
	} else if strings.EqualFold(pusher.VCodec(), "h265") {
		if len(rtp.Payload) >= 3 {
			firstByte := rtp.Payload[0]
			headerType := (firstByte >> 1) & 0x3f
			var frameType uint8
			if headerType == 49 { //Fragmentation Units

				FUHeader := rtp.Payload[2]
				/*
				   +---------------+
				   |0|1|2|3|4|5|6|7|
				   +-+-+-+-+-+-+-+-+
				   |S|E|  FuType   |
				   +---------------+
				*/
				rtpStart := (FUHeader & 0x80) != 0
				if !rtpStart {
					if (FUHeader & 0x40) != 0 {
						//log.Printf("FU frame end")
					}
					return false
				} else {
					//log.Printf("FU frame start")
				}
				frameType = FUHeader & 0x3f
			} else if headerType == 48 { //Aggregation Packets

			} else if headerType == 50 { //PACI Packets

			} else { // Single NALU
				/*
					+---------------+---------------+
					|0|1|2|3|4|5|6|7|0|1|2|3|4|5|6|7|
					+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
					|F|   Type    |  LayerId  | TID |
					+-------------+-----------------+
				*/
				frameType = firstByte & 0x7e
			}
			if frameType >= 16 && frameType <= 21 {
				return true
			}
			if frameType == 32 {
				// vps sps pps...
				if len(rtp.Payload) < 200 { // consider sps pps header only.
					return false
				}
				return true
			}
		}
		return false
	}
	return false
}
