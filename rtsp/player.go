package rtsp

import (
	"time"
	"github.com/EasyDarwin/EasyGoLib/utils"
)

// Player wrapper of Media data receiver
type Player interface {
	// usage
	QueueRTP(pack *RTPPack) Player
	Start()
	Stop()
	// status
	ID() string
	Path() string
	TransType() TransType
	InBytes() uint
	OutBytes() uint
	StartAt() time.Time
}

type _Player struct {
	*Session
	Pusher Pusher
	queue  chan *RTPPack
}

// NewPlayer of network session
func NewPlayer(session *Session, pusher Pusher) Player {
	player := &_Player{
		Session: session,
		Pusher:  pusher,
		queue:   make(chan *RTPPack, config.Player.SendQueueLength),
	}
	session.StopHandles = append(session.StopHandles, func() {
		pusher.RemovePlayer(player)
		close(player.queue)
	})
	return player
}

func (player *_Player) ID() string {
	return player.Session.ID
}

func (player *_Player) Path() string {
	return player.Session.URL
}

func (player *_Player) TransType() TransType {
	return player.Session.TransType
}

func (player *_Player) InBytes() uint {
	return player.Session.InBytes
}

func (player *_Player) OutBytes() uint {
	return player.Session.OutBytes
}

func (player *_Player) StartAt() time.Time {
	return player.Session.StartAt
}

func (player *_Player) QueueRTP(pack *RTPPack) Player {
	if pack == nil {
		log.Debug("player queue enter nil pack, drop it")
		return player
	}
	select {
	case player.queue <- pack:
	default:
		log.Infof("player[%s] queue full, drop it", player.ID())
	}
	return player
}

func (player *_Player) Start() {
	timer := time.Unix(0, 0)
	var pack *RTPPack
	var ok bool
	for !player.Stoped {
		pack, ok = <-player.queue
		if !ok {
			log.Infof("player[%s] send queue stopped, quit send loop", player.ID())
			return
		}
		if pack == nil {
			if !player.Stoped {
				log.Error("Player[%s] not stoped, but queue take out nil pack", player.ID())
			}
			continue
		}
		if err := player.Session.SendRTP(pack); err != nil {
			log.Error(err)
		}
		elapsed := time.Now().Sub(timer)
		if elapsed >= 30*time.Second {
			log.Debugf("Send a package.type:%d\n", pack.Type)
			timer = time.Now()
		}
	}
}

func (player *_Player) Stop() {
	player.Session.Stop()
}

func (player *Player) Pause(paused bool) {
	if paused {
		player.logger.Printf("Player %s, Pause\n", player.String())
	} else {
		player.logger.Printf("Player %s, Play\n", player.String())
	}
	player.cond.L.Lock()
	if paused && player.dropPacketWhenPaused && len(player.queue) > 0 {
		player.queue = make([]*RTPPack, 0)
	}
	player.paused = paused
	player.cond.L.Unlock()
}