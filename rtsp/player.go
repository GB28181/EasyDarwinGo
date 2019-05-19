package rtsp

import (
	"time"

	"github.com/penggy/EasyGoLib/utils"
)

type Player struct {
	*Session
	Pusher *Pusher
	queue  chan *RTPPack
}

func NewPlayer(session *Session, pusher *Pusher) (player *Player) {
	player = &Player{
		Session: session,
		Pusher:  pusher,
		queue:   make(chan *RTPPack, utils.Conf().Section("rtp").Key("send_queue_length").MustInt(128)),
	}
	session.StopHandles = append(session.StopHandles, func() {
		pusher.RemovePlayer(player)
		close(player.queue)
	})
	return
}

func (player *Player) QueueRTP(pack *RTPPack) *Player {
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

func (player *Player) Start() {
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
