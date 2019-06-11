package rtsp

import (
	"fmt"

	"github.com/EasyDarwin/EasyDarwin/utils"
	"github.com/benbjohnson/immutable"
)

type defaultPusher struct {
	server *Server

	players       *immutable.Map //SessionID <-> Player
	playersLocker *utils.SpinLock

	outBytes uint
	inBytes  uint

	StopHandles []func()
}

func newDefaultPusher(server *Server) *defaultPusher {
	return &defaultPusher{
		server: server,

		players:       immutable.NewMap(nil),
		playersLocker: &utils.SpinLock{},
	}
}

func (pusher *defaultPusher) Server() *Server {
	return pusher.server
}

func (pusher *defaultPusher) GetPlayers() *immutable.Map {
	pusher.playersLocker.Lock()
	players := pusher.players
	pusher.playersLocker.Unlock()

	return players
}

func (pusher *defaultPusher) AddOutputBytes(size int) {
	pusher.outBytes += uint(size)
}

// InBytes of VOD
func (pusher *defaultPusher) InBytes() uint {
	return pusher.inBytes
}

// OutBytes of VOD
func (pusher *defaultPusher) OutBytes() uint {
	return pusher.outBytes
}

func (pusher *defaultPusher) BroadcastRTP(packet *RTPPack) {
	players := pusher.GetPlayers()
	for itPlayer := players.Iterator(); !itPlayer.Done(); {
		_, _player := itPlayer.Next()
		player := _player.(Player)
		player.QueueRTP(packet)
		pusher.AddOutputBytes(packet.Buffer.Len())
	}
}

func (pusher *defaultPusher) ClearPlayer() {
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

func (pusher *defaultPusher) GetPlayer(ID string) Player {
	_player, ok := pusher.players.Get(ID)
	if !ok {
		return nil
	}

	return _player.(Player)
}

func (pusher *defaultPusher) AddPlayer(player Player) error {
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

func (pusher *defaultPusher) RemovePlayer(player Player) {
	pusher.playersLocker.Lock()
	pusher.players = pusher.players.Delete(player.ID())
	pusher.playersLocker.Unlock()

	log.Infof("player %s end, now player size[%d]\n", player.ID(), pusher.players.Len())
}
