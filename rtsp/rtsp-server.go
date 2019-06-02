package rtsp

import (
	"fmt"
	"net"
	"sync"
)

// OnGetPusherHandle calls when server was called GetPusher(path)
type OnGetPusherHandle func(_ *Server, _ *Session, path string, _ Pusher) Pusher

// Server of RTSP
type Server struct {
	TCPListener    *net.TCPListener
	TCPPort        int
	Stoped         bool
	pushers        map[string]Pusher // Path <-> Pusher
	pushersLock    sync.RWMutex
	players        map[string]Player
	playersLock    sync.RWMutex
	addPusherCh    chan Pusher
	removePusherCh chan Pusher
	// Hooks
	onGetPusherHandles []OnGetPusherHandle
}

// Instance of RTSP server
var Instance *Server

func initServer() error {
	Instance = &Server{
		Stoped:         true,
		TCPPort:        config.RTSP.Port,
		pushers:        make(map[string]Pusher),
		addPusherCh:    make(chan Pusher),
		removePusherCh: make(chan Pusher),
	}

	return nil
}

func GetServer() *Server {
	return Instance
}

func (server *Server) pusherHooks() {
	for {
		select {
		case _, ok := <-server.addPusherCh:
			if !ok {
				return
			}
		case _, ok := <-server.removePusherCh:
			if !ok {
				return
			}
		}
	}
}

func (server *Server) Start() (err error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", server.TCPPort))
	if err != nil {
		return
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return
	}

	go server.pusherHooks()

	server.Stoped = false
	server.TCPListener = listener
	log.Infof("RTSP server start on[%d]", server.TCPPort)
	networkBuffer := config.RTSP.NetworkBuffer
	for !server.Stoped {
		conn, err := server.TCPListener.Accept()
		if err != nil {
			if !server.Stoped {
				log.Errorf("RTSP server listen fail:[%v]", err)
			}
			continue
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := tcpConn.SetReadBuffer(networkBuffer); err != nil {
				log.Errorf("RTSP server conn set read buffer error, %v", err)
			}
			if err := tcpConn.SetWriteBuffer(networkBuffer); err != nil {
				log.Errorf("RTSP server conn set write buffer error, %v", err)
			}
		}

		session := NewSession(server, conn)
		go session.Start()
	}
	return
}

func (server *Server) Stop() {
	log.Infof("rtsp server stop on %d", server.TCPPort)
	server.Stoped = true
	if server.TCPListener != nil {
		server.TCPListener.Close()
		server.TCPListener = nil
	}
	server.pushersLock.Lock()
	server.pushers = make(map[string]Pusher)
	server.pushersLock.Unlock()

	close(server.addPusherCh)
	close(server.removePusherCh)
}

func (server *Server) AddPusher(pusher Pusher, closeOld bool) bool {
	added := false
	server.pushersLock.Lock()
	old, ok := server.pushers[pusher.Path()]
	if !ok {
		server.pushers[pusher.Path()] = pusher
		log.Infof("%v start, now pusher size[%d]", pusher, len(server.pushers))
		added = true
	} else {
		if closeOld {
			server.pushers[pusher.Path()] = pusher
			log.Infof("%v start, replace old pusher", pusher)
			added = true
		}
	}
	server.pushersLock.Unlock()
	if ok && closeOld {
		log.Infof("old pusher %v stoped", pusher)
		old.Stop()
		server.removePusherCh <- old
	}
	if added {
		go pusher.Start()
		server.addPusherCh <- pusher
	}
	return added
}

// RemovePusher from RTSP server
func (server *Server) RemovePusher(pusher Pusher) {
	removed := false
	server.pushersLock.Lock()
	if _pusher, ok := server.pushers[pusher.Path()]; ok && pusher.ID() == _pusher.ID() {
		delete(server.pushers, pusher.Path())
		log.Infof("%v end, now pusher size[%d]\n", pusher, len(server.pushers))
		removed = true
	}
	server.pushersLock.Unlock()
	if removed {
		server.removePusherCh <- pusher
	}
}

// AddOnGetPusherHandle to RTSP server
func (server *Server) AddOnGetPusherHandle(handle OnGetPusherHandle) {
	// TODO: handle state change func like windows message
	server.onGetPusherHandles = append(server.onGetPusherHandles, handle)
}

// GetPusher according to path of request
// pass session for dynamic create pusher lifecycle or other necessary reseaons
func (server *Server) GetPusher(path string, session *Session) (pusher Pusher) {
	server.pushersLock.RLock()
	pusher = server.pushers[path]
	server.pushersLock.RUnlock()

	for _, handle := range server.onGetPusherHandles {
		pusher = handle(server, session, path, pusher)
	}

	return
}

func (server *Server) GetPushers() (pushers map[string]Pusher) {
	pushers = make(map[string]Pusher)
	server.pushersLock.RLock()
	for k, v := range server.pushers {
		pushers[k] = v
	}
	server.pushersLock.RUnlock()
	return
}

func (server *Server) GetPusherSize() (size int) {
	server.pushersLock.RLock()
	size = len(server.pushers)
	server.pushersLock.RUnlock()
	return
}
