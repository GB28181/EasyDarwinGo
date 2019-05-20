package rtsp

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/penggy/EasyGoLib/utils"
)

type UDPServer struct {
	*Session
	*RTSPClient

	APort        []int
	AConn        []*net.UDPConn
	AControlPort []int
	AControlConn []*net.UDPConn
	VPort        int
	VConn        *net.UDPConn
	VControlPort int
	VControlConn *net.UDPConn

	Stoped bool
}

func NewUDPServerFromClient(client *RTSPClient) *UDPServer {
	return &UDPServer{
		RTSPClient:   client,
		APort:        []int{-1, -1},
		AConn:        []*net.UDPConn{nil, nil},
		AControlPort: []int{-1, -1},
		AControlConn: []*net.UDPConn{nil, nil},
	}
}

func NewUDPServerFromSession(session *Session) *UDPServer {
	return &UDPServer{
		Session:      session,
		APort:        []int{-1, -1},
		AConn:        []*net.UDPConn{nil, nil},
		AControlPort: []int{-1, -1},
		AControlConn: []*net.UDPConn{nil, nil},
	}
}

func (s *UDPServer) AddInputBytes(bytes int) {
	if s.Session != nil {
		s.Session.InBytes += bytes
		return
	}
	if s.RTSPClient != nil {
		s.RTSPClient.InBytes += bytes
		return
	}
	panic(fmt.Errorf("session and RTSPClient both nil"))
}

func (s *UDPServer) HandleRTP(pack *RTPPack) {
	if s.Session != nil {
		for _, v := range s.Session.RTPHandles {
			v(pack)
		}
		return
	}

	if s.RTSPClient != nil {
		for _, v := range s.RTSPClient.RTPHandles {
			v(pack)
		}
		return
	}
	panic(fmt.Errorf("session and RTSPClient both nil"))
}

func (s *UDPServer) Logger() *log.Logger {
	if s.Session != nil {
		return s.Session.logger
	}
	if s.RTSPClient != nil {
		return s.RTSPClient.logger
	}
	panic(fmt.Errorf("session and RTSPClient both nil"))
}

func (s *UDPServer) Stop() {
	if s.Stoped {
		return
	}
	s.Stoped = true
	for i := range s.AConn {
		if s.AConn[i] != nil {
			s.AConn[i].Close()
			s.AConn[i] = nil
		}
	}
	for i := range s.AControlConn {
		if s.AControlConn[i] != nil {
			s.AControlConn[i].Close()
			s.AControlConn[i] = nil
		}
	}
	if s.VConn != nil {
		s.VConn.Close()
		s.VConn = nil
	}
	if s.VControlConn != nil {
		s.VControlConn.Close()
		s.VControlConn = nil
	}
}

func (s *UDPServer) SetupAudio(aChannel int) (err error) {
	logger := s.Logger()
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return
	}
	s.AConn[aChannel], err = net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	networkBuffer := utils.Conf().Section("rtsp").Key("network_buffer").MustInt(1048576)
	if err := s.AConn[aChannel].SetReadBuffer(networkBuffer); err != nil {
		logger.Printf("udp server audio conn set read buffer error, %v", err)
	}
	if err := s.AConn[aChannel].SetWriteBuffer(networkBuffer); err != nil {
		logger.Printf("udp server audio conn set write buffer error, %v", err)
	}
	la := s.AConn[aChannel].LocalAddr().String()
	strPort := la[strings.LastIndex(la, ":")+1:]
	s.APort[aChannel], err = strconv.Atoi(strPort)
	if err != nil {
		return
	}
	go func(aChannel int) {
		bufUDP := make([]byte, UDP_BUF_SIZE)
		logger.Printf("udp server start listen audio port[%d]", s.APort)
		defer logger.Printf("udp server stop listen audio port[%d]", s.APort)
		timer := time.Unix(0, 0)
		for !s.Stoped {
			if n, _, err := s.AConn[aChannel].ReadFromUDP(bufUDP); err == nil {
				elapsed := time.Now().Sub(timer)
				if elapsed >= 30*time.Second {
					logger.Printf("Package recv from AConn.len:%d\n", n)
					timer = time.Now()
				}
				rtpBytes := make([]byte, n)
				s.AddInputBytes(n)
				copy(rtpBytes, bufUDP)
				pack := &RTPPack{
					Type:    RTP_TYPE_AUDIO,
					Buffer:  bytes.NewBuffer(rtpBytes),
					Channel: aChannel,
				}
				s.HandleRTP(pack)
			} else {
				logger.Printf("udp server[%d] read audio pack error:%v",
					s.APort, err)
				continue
			}
		}
	}(aChannel)
	addr, err = net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return
	}
	s.AControlConn[aChannel], err = net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	if err := s.AControlConn[aChannel].SetReadBuffer(networkBuffer); err != nil {
		logger.Printf("udp server audio control conn set read buffer error, %v", err)
	}
	if err := s.AControlConn[aChannel].SetWriteBuffer(networkBuffer); err != nil {
		logger.Printf("udp server audio control conn set write buffer error, %v", err)
	}
	la = s.AControlConn[aChannel].LocalAddr().String()
	strPort = la[strings.LastIndex(la, ":")+1:]
	s.AControlPort[aChannel], err = strconv.Atoi(strPort)
	if err != nil {
		return
	}
	go func(aChannel int) {
		bufUDP := make([]byte, UDP_BUF_SIZE)
		logger.Printf("udp server start listen audio control port[%d]", s.AControlPort)
		defer logger.Printf("udp server stop listen audio control port[%d]", s.AControlPort)
		for !s.Stoped {
			if n, _, err := s.AControlConn[aChannel].ReadFromUDP(bufUDP); err == nil {
				//logger.Printf("Package recv from AControlConn.len:%d\n", n)
				rtpBytes := make([]byte, n)
				s.AddInputBytes(n)
				copy(rtpBytes, bufUDP)
				pack := &RTPPack{
					Type:   RTP_TYPE_AUDIOCONTROL,
					Buffer: bytes.NewBuffer(rtpBytes),
				}
				s.HandleRTP(pack)
			} else {
				logger.Println("udp server read audio control pack error", err)
				continue
			}
		}
	}(aChannel)
	return
}

func (s *UDPServer) SetupVideo() (err error) {
	logger := s.Logger()
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return
	}
	s.VConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	networkBuffer := utils.Conf().Section("rtsp").Key("network_buffer").MustInt(1048576)
	if err := s.VConn.SetReadBuffer(networkBuffer); err != nil {
		logger.Printf("udp server video conn set read buffer error, %v", err)
	}
	if err := s.VConn.SetWriteBuffer(networkBuffer); err != nil {
		logger.Printf("udp server video conn set write buffer error, %v", err)
	}
	la := s.VConn.LocalAddr().String()
	strPort := la[strings.LastIndex(la, ":")+1:]
	s.VPort, err = strconv.Atoi(strPort)
	if err != nil {
		return
	}
	go func() {
		bufUDP := make([]byte, UDP_BUF_SIZE)
		logger.Printf("udp server start listen video port[%d]", s.VPort)
		defer logger.Printf("udp server stop listen video port[%d]", s.VPort)
		timer := time.Unix(0, 0)
		for !s.Stoped {
			if n, _, err := s.VConn.ReadFromUDP(bufUDP); err == nil {
				elapsed := time.Now().Sub(timer)
				if elapsed >= 30*time.Second {
					logger.Printf("Package recv from VConn.len:%d\n", n)
					timer = time.Now()
				}
				rtpBytes := make([]byte, n)
				s.AddInputBytes(n)
				copy(rtpBytes, bufUDP)
				pack := &RTPPack{
					Type:   RTP_TYPE_VIDEO,
					Buffer: bytes.NewBuffer(rtpBytes),
				}
				s.HandleRTP(pack)
			} else {
				logger.Println("udp server read video pack error", err)
				continue
			}
		}
	}()

	addr, err = net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return
	}
	s.VControlConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	if err := s.VControlConn.SetReadBuffer(networkBuffer); err != nil {
		logger.Printf("udp server video control conn set read buffer error, %v", err)
	}
	if err := s.VControlConn.SetWriteBuffer(networkBuffer); err != nil {
		logger.Printf("udp server video control conn set write buffer error, %v", err)
	}
	la = s.VControlConn.LocalAddr().String()
	strPort = la[strings.LastIndex(la, ":")+1:]
	s.VControlPort, err = strconv.Atoi(strPort)
	if err != nil {
		return
	}
	go func() {
		bufUDP := make([]byte, UDP_BUF_SIZE)
		logger.Printf("udp server start listen video control port[%d]", s.VControlPort)
		defer logger.Printf("udp server stop listen video control port[%d]", s.VControlPort)
		for !s.Stoped {
			if n, _, err := s.VControlConn.ReadFromUDP(bufUDP); err == nil {
				//logger.Printf("Package recv from VControlConn.len:%d\n", n)
				rtpBytes := make([]byte, n)
				s.AddInputBytes(n)
				copy(rtpBytes, bufUDP)
				pack := &RTPPack{
					Type:   RTP_TYPE_VIDEOCONTROL,
					Buffer: bytes.NewBuffer(rtpBytes),
				}
				s.HandleRTP(pack)
			} else {
				logger.Println("udp server read video control pack error", err)
				continue
			}
		}
	}()
	return
}
