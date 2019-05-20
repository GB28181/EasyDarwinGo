package rtsp

import (
	"time"

	"github.com/penggy/EasyGoLib/utils"
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
	InBytes() int
	OutBytes() int
	StartAt() time.Time
}

type _Player struct {
	*Session
	Pusher *Pusher
	queue  chan *RTPPack
}

// NewPlayer of network session
func NewPlayer(session *Session, pusher *Pusher) Player {
	player := &_Player{
		Session: session,
		Pusher:  pusher,
		queue:   make(chan *RTPPack, utils.Conf().Section("rtp").Key("send_queue_length").MustInt(128)),
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

func (player *_Player) InBytes() int {
	return player.Session.InBytes
}

func (player *_Player) OutBytes() int {
	return player.Session.OutBytes
}

func (player *_Player) StartAt() time.Time {
	return player.Session.StartAt
}

func (player *_Player) QueueRTP(pack *RTPPack) Player {
	logger := player.logger
	if pack == nil {
		logger.Printf("player queue enter nil pack, drop it")
		return player
	}
	select {
	case player.queue <- pack:
	default:
		logger.Printf("player queue full, drop it")
	}
	return player
}

func (player *_Player) Start() {
	logger := player.logger
	timer := time.Unix(0, 0)
	var pack *RTPPack
	var ok bool
	for !player.Stoped {
		pack, ok = <-player.queue
		if !ok {
			logger.Printf("player send queue stopped, quit send loop")
			return
		}
		if pack == nil {
			if !player.Stoped {
				logger.Printf("player not stoped, but queue take out nil pack")
			}
			continue
		}
		if err := player.SendRTP(pack); err != nil {
			logger.Println(err)
		}
		elapsed := time.Now().Sub(timer)
		if elapsed >= 30*time.Second {
			logger.Printf("Send a package.type:%d\n", pack.Type)
			timer = time.Now()
		}
	}
}

func (player *_Player) Stop() {
	player.Session.Stop()
}
