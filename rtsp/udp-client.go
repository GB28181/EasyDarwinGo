package rtsp

import (
	"fmt"
	"net"
	"strings"
)

type UDPClient struct {
	*Session

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

// NewUDPClient returns
func NewUDPClient(session *Session) *UDPClient {
	return &UDPClient{
		Session:      session,
		APort:        []int{-1, -1},
		AConn:        []*net.UDPConn{nil, nil},
		AControlPort: []int{-1, -1},
		AControlConn: []*net.UDPConn{nil, nil},
	}
}

func (s *UDPClient) Stop() {
	if s.Stoped {
		return
	}
	s.Stoped = true
	for i := range s.AConn {
		if nil != s.AConn[i] {
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

func (c *UDPClient) SetupAudio(aChannel int) (err error) {
	defer func() {
		if err != nil {
			log.Error(err)
			c.Stop()
		}
	}()
	host := c.Conn.RemoteAddr().String()
	host = host[:strings.LastIndex(host, ":")]
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, c.APort[aChannel]))
	if err != nil {
		return
	}
	c.AConn[aChannel], err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	networkBuffer := config.RTSP.NetworkBuffer
	if err := c.AConn[aChannel].SetReadBuffer(networkBuffer); err != nil {
		log.Errorf("udp client audio conn set read buffer error, %v", err)
	}
	if err := c.AConn[aChannel].SetWriteBuffer(networkBuffer); err != nil {
		log.Errorf("udp client audio conn set write buffer error, %v", err)
	}

	addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, c.AControlPort[aChannel]))
	if err != nil {
		return
	}
	c.AControlConn[aChannel], err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	if err := c.AControlConn[aChannel].SetReadBuffer(networkBuffer); err != nil {
		log.Errorf("udp client audio control conn set read buffer error, %v", err)
	}
	if err := c.AControlConn[aChannel].SetWriteBuffer(networkBuffer); err != nil {
		log.Errorf("udp client audio control conn set write buffer error, %v", err)
	}
	return
}

func (c *UDPClient) SetupVideo() (err error) {
	defer func() {
		if err != nil {
			log.Error(err)
			c.Stop()
		}
	}()
	host := c.Conn.RemoteAddr().String()
	host = host[:strings.LastIndex(host, ":")]
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, c.VPort))
	if err != nil {
		return
	}
	c.VConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	networkBuffer := config.RTSP.NetworkBuffer
	if err := c.VConn.SetReadBuffer(networkBuffer); err != nil {
		log.Errorf("udp client video conn set read buffer error, %v", err)
	}
	if err := c.VConn.SetWriteBuffer(networkBuffer); err != nil {
		log.Errorf("udp client video conn set write buffer error, %v", err)
	}

	addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, c.VControlPort))
	if err != nil {
		return
	}
	c.VControlConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	if err := c.VControlConn.SetReadBuffer(networkBuffer); err != nil {
		log.Errorf("udp client video control conn set read buffer error, %v", err)
	}
	if err := c.VControlConn.SetWriteBuffer(networkBuffer); err != nil {
		log.Errorf("udp client video control conn set write buffer error, %v", err)
	}
	return
}

func (c *UDPClient) SendRTP(pack *RTPPack) (err error) {
	if pack == nil {
		err = fmt.Errorf("udp client send rtp got nil pack")
		return
	}
	var conn *net.UDPConn
	switch pack.Type {
	case RTP_TYPE_AUDIO:
		conn = c.AConn[pack.Channel]
	case RTP_TYPE_AUDIOCONTROL:
		conn = c.AControlConn[pack.Channel]
	case RTP_TYPE_VIDEO:
		conn = c.VConn
	case RTP_TYPE_VIDEOCONTROL:
		conn = c.VControlConn
	default:
		err = fmt.Errorf("udp client send rtp got unkown pack type[%v]", pack.Type)
		return
	}
	if conn == nil {
		// It could be client not setting up all media channel
		// For efficient , not format error
		// err = fmt.Errorf("udp client send rtp pack type[%v] failed, conn not found", pack.Type)
		return nil
	}
	n, err := conn.Write(pack.Buffer.Bytes())
	if err != nil {
		err = fmt.Errorf("udp client write bytes error, %v", err)
		return
	}
	// logger.Printf("udp client write [%d/%d]", n, pack.Buffer.Len())
	c.Session.OutBytes += uint(n)
	return
}
