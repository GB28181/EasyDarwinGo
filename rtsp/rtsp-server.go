package rtsp

import (
	"fmt"
	"net"
	"sync"

	"github.com/benbjohnson/immutable"
)

// OnGetPusherHandle calls when server was called GetPusher(path)
type OnGetPusherHandle func(_ *Server, _ *Session, path string, _ Pusher) Pusher

// Server of RTSP
type Server struct {
	Listener       net.Listener
	TCPListener    *net.TCPListener
	TCPPort        int
	Stoped         bool
	players        map[string]Player
	playersLock    sync.RWMutex
	addPusherCh    chan Pusher
	removePusherCh chan Pusher
	// Hooks
	onGetPusherHandles []OnGetPusherHandle
	// Pushers
	pushers              *immutable.Map
	getPushers           chan *immutable.Map
	pusherCommandChannel chan func()
}

// Instance of RTSP server
var Instance *Server

func initServer() error {
	Instance = &Server{
		Stoped:         true,
		TCPPort:        config.RTSP.Port,
		addPusherCh:    make(chan Pusher),
		removePusherCh: make(chan Pusher),
		// pushers will init when start to make sure a clean start
	}

	return nil
}

// GetServer of RTSP
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

func (server *Server) pusherLoop() {

	for {
		select {
		case do, ok := <-server.pusherCommandChannel:
			if ok {
				do()

			CleanCacheLoop:
				for {
					select {
					case _ = <-server.getPushers:
					default:
						break CleanCacheLoop
					}
				}
			} else {
				return
			}
		case server.getPushers <- server.pushers:
		}
	}
}

func (server *Server) initPushers() {
	server.pushers = immutable.NewMap(nil)
	server.getPushers = make(chan *immutable.Map, 16)
	server.pusherCommandChannel = make(chan func(), 16)

	go server.pusherLoop()
}

func (server *Server) _finishPushers() {
	// TODO: stop all pushers
	close(server.pusherCommandChannel)
	server.pushers = immutable.NewMap(nil)
}

func (server *Server) finishPushers() {
	server.pusherCommandChannel <- server._finishPushers
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
	server.initPushers()

	server.Stoped = false
	server.Listener = listener
	log.Infof("RTSP server start on[%d]", server.TCPPort)
	networkBuffer := config.RTSP.NetworkBuffer
	for !server.Stoped {
		conn, err := server.Listener.Accept()
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

// Stop RTSP server
func (server *Server) Stop() {
	log.Infof("rtsp server stop on %d", server.TCPPort)
	server.Stoped = true
	if server.Listener != nil {
		server.Listener.Close()
		server.Listener = nil
	}
	server.finishPushers()

	close(server.addPusherCh)
	close(server.removePusherCh)
}

type serverAddPusherCommand struct {
	server   *Server
	pusher   Pusher
	closeOld bool
	result   chan bool
}

func (c *serverAddPusherCommand) Do() {
	added := false

	_old, existOld := c.server.pushers.Get(c.pusher.Path())
	oldPusher, _ := _old.(Pusher)

	if !existOld || c.closeOld {
		c.server.pushers = c.server.pushers.Set(c.pusher.Path(), c.pusher)
		added = true
		log.Infof("pusher %s added, now pusher size[%d]", c.pusher.ID(), c.server.pushers.Len())
	}

	if existOld && c.closeOld {
		go oldPusher.Stop()
	}

	if added {
		go c.pusher.Start()
	}

	c.result <- added

func (server *Server) TryAttachToPusher(session *Session) (int, *Pusher) {
	server.pushersLock.Lock()
	attached := 0
	var pusher *Pusher = nil
	if _pusher, ok := server.pushers[session.Path]; ok {
		if _pusher.RebindSession(session) {
			session.logger.Printf("Attached to a pusher")
			attached = 1
			pusher = _pusher
		} else {
			attached = -1
		}
	}
	server.pushersLock.Unlock()
	return attached, pusher
}

// AddPusher to Server
func (server *Server) AddPusher(pusher Pusher, closeOld bool) bool {
	cmd := &serverAddPusherCommand{
		server:   server,
		pusher:   pusher,
		closeOld: closeOld,
		result:   make(chan bool, 1),
	}

	server.pusherCommandChannel <- cmd.Do

	return <-cmd.result
}

type serverRemovePusherCommmand struct {
	server *Server
	ID     string
	result chan int
}

func (c *serverRemovePusherCommmand) Do() {
	_pusher, exist := c.server.pushers.Get(c.ID)
	if exist {
		c.server.pushers = c.server.pushers.Delete(c.ID)
		log.Infof("pusher [%s] removed, now pusher size[%d]\n", c.ID, c.server.pushers.Len())

		go _pusher.(Pusher).Stop()
	}

	c.result <- 1
}

type serverRemovePusherCommmand struct {
	server *Server
	ID     string
	result chan int
}

func (c *serverRemovePusherCommmand) Do() {
	_pusher, exist := c.server.pushers.Get(c.ID)
	if exist {
		c.server.pushers = c.server.pushers.Delete(c.ID)
		log.Infof("pusher [%s] removed, now pusher size[%d]\n", c.ID, c.server.pushers.Len())

		go _pusher.(Pusher).Stop()
	}

	c.result <- 1
}

// AddOnGetPusherHandle to RTSP server
func (server *Server) AddOnGetPusherHandle(handle OnGetPusherHandle) {
	// TODO: handle state change func like windows message
	server.onGetPusherHandles = append(server.onGetPusherHandles, handle)
}

// GetPusher according to path of request
// pass session for dynamic create pusher lifecycle or other necessary reseaons
func (server *Server) GetPusher(path string, session *Session) (pusher Pusher) {
	_pusher, ok := server.GetPushers().Get(path)
	if ok {
		pusher = _pusher.(Pusher)
	}

	for _, handle := range server.onGetPusherHandles {
		pusher = handle(server, session, path, pusher)
	}

	return
}

// RemovePusher from RTSP server
func (server *Server) RemovePusher(ID string) {
	cmd := &serverRemovePusherCommmand{
		server: server,
		ID:     ID,
		result: make(chan int, 1),
	}

	server.pusherCommandChannel <- cmd.Do

	<-cmd.result
}

// AddOnGetPusherHandle to RTSP server
func (server *Server) AddOnGetPusherHandle(handle OnGetPusherHandle) {
	// TODO: handle state change func like windows message
	server.onGetPusherHandles = append(server.onGetPusherHandles, handle)
}

// GetPusher according to path of request
// pass session for dynamic create pusher lifecycle or other necessary reseaons
func (server *Server) GetPusher(path string, session *Session) (pusher Pusher) {
	_pusher, ok := server.GetPushers().Get(path)
	if ok {
		pusher = _pusher.(Pusher)
	}

	for _, handle := range server.onGetPusherHandles {
		pusher = handle(server, session, path, pusher)
	}

	return
}


// GetPushers from RTSP server
func (server *Server) GetPushers() *immutable.Map {
	return <-server.getPushers
}

// GetPusherSize from RTSP server
func (server *Server) GetPusherSize() int {
	return server.GetPushers().Len()
}
