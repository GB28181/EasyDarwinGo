package rtsp

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pixelbender/go-sdp/sdp"

	"github.com/teris-io/shortid"
)

type SessionType int

const (
	SESSION_TYPE_PUSHER SessionType = iota
	SESSEION_TYPE_PLAYER
)

func (st SessionType) String() string {
	switch st {
	case SESSION_TYPE_PUSHER:
		return "pusher"
	case SESSEION_TYPE_PLAYER:
		return "player"
	}
	return "unknow"
}

type RTPType uint8

const (
	RTP_TYPE_AUDIO RTPType = iota
	RTP_TYPE_VIDEO
	RTP_TYPE_AUDIOCONTROL
	RTP_TYPE_VIDEOCONTROL
)

func (rt RTPType) String() string {
	switch rt {
	case RTP_TYPE_AUDIO:
		return "audio"
	case RTP_TYPE_VIDEO:
		return "video"
	case RTP_TYPE_AUDIOCONTROL:
		return "audio control"
	case RTP_TYPE_VIDEOCONTROL:
		return "video control"
	}
	return "unknow"
}

type TransType int

const (
	TRANS_TYPE_TCP TransType = iota
	TRANS_TYPE_UDP
	TRANS_TYPE_INTERNAL
)

func (tt TransType) String() string {
	switch tt {
	case TRANS_TYPE_TCP:
		return "TCP"
	case TRANS_TYPE_UDP:
		return "UDP"
	case TRANS_TYPE_INTERNAL:
		return "Internal"
	}
	return "unknow"
}

const UDP_BUF_SIZE = 1048576

type Session struct {
	ID        string
	Server    *Server
	Conn      *RichConn
	connRW    *bufio.ReadWriter
	connWLock sync.RWMutex
	Type      SessionType
	TransType TransType
	Path      string
	URL       string
	SDPRaw    string
	Sdp       *sdp.Session

	authorizationEnable bool
	nonce               string
	closeOld            bool
	debugLogEnable      bool
	streamSecret        string

	AControl []string
	VControl string
	ACodec   []string
	VCodec   string

	// stats info
	InBytes  uint
	OutBytes uint
	StartAt  time.Time
	Timeout  int

	Stoped     bool
	stopedLock sync.RWMutex

	// TCP channels
	aRTPChannel        []int
	aRTPControlChannel []int
	vRTPChannel        int
	vRTPControlChannel int

	// audio double channel
	aChannelNum int

	Pusher      Pusher
	Player      Player
	UDPClient   *UDPClient
	UDPServer   *UDPServer
	RTPHandles  []func(*RTPPack)
	StopHandles []func()
}

func (session *Session) String() string {
	return fmt.Sprintf("session[%v][%v][%s][%s][%s]", session.Type, session.TransType, session.Path, session.ID, session.Conn.RemoteAddr().String())
}

// NewSession returns
func NewSession(server *Server, conn net.Conn) *Session {
	// networkBuffer := utils.Conf().Section("rtsp").Key("network_buffer").MustInt(204800)
	// timeoutMillis := utils.Conf().Section("rtsp").Key("timeout").MustInt(0)
	// authorizationEnable := utils.Conf().Section("rtsp").Key("authorization_enable").MustInt(0)
	// closeOld := utils.Conf().Section("rtsp").Key("close_old").MustInt(0)

	networkBuffer := config.RTSP.NetworkBuffer
	timeoutMillis := config.RTSP.Timeout
	authorizationEnable := config.RTSP.AuthorizationEnable
	closeOld := config.RTSP.CloseOld

	timeoutTCPConn := &RichConn{conn, time.Duration(timeoutMillis) * time.Millisecond}
	debugLogEnable := utils.Conf().Section("rtsp").Key("debug_log_enable").MustInt(0)
	session := &Session{
		ID:     shortid.MustGenerate(),
		Server: server,
		Conn:   timeoutTCPConn,
		connRW: bufio.NewReadWriter(
			bufio.NewReaderSize(timeoutTCPConn, networkBuffer),
			bufio.NewWriterSize(timeoutTCPConn, networkBuffer),
		),
		StartAt:             time.Now(),
		Timeout:             config.RTSP.Timeout,
		authorizationEnable: authorizationEnable != 0,
		debugLogEnable:      debugLogEnable != 0,
		RTPHandles:          make([]func(*RTPPack), 0),
		StopHandles:         make([]func(), 0),
		vRTPChannel:         -1,
		vRTPControlChannel:  -1,
		aRTPChannel:         []int{-1, -1},
		aRTPControlChannel:  []int{-1, -1},
		closeOld:            closeOld != 0,
		streamSecret:        server.streamSecret,
	}

	return session
}

// Stop the RTSP session
func (session *Session) Stop() {
	if session.getStoped() {
	// TODO: more strong safe guard
		return
	}
	session.setStoped(true)

	for _, h := range session.StopHandles {
		h()
	}
	if session.Conn != nil {
		session.connRW.Flush()
		session.Conn.Close()
		session.Conn = nil
	}
	if session.UDPClient != nil {
		session.UDPClient.Stop()
		session.UDPClient = nil
	}
	if session.UDPServer != nil {
		session.UDPServer.Stop()
		session.UDPServer = nil
	}
}

func (session *Session) Start() {
	defer session.Stop()

	buf1 := make([]byte, 1)
	buf2 := make([]byte, 2)
	timer := time.Unix(0, 0)
	for !session.getStoped() {
		if _, err := io.ReadFull(session.connRW, buf1); err != nil {
			log.Errorf("%s:%v", session, err)
			return
		}
		if buf1[0] == 0x24 { //rtp data
			if _, err := io.ReadFull(session.connRW, buf1); err != nil {
				log.Errorf("%s:%v", session, err)
				return
			}
			if _, err := io.ReadFull(session.connRW, buf2); err != nil {
				log.Errorf("%s:%v", session, err)
				return
			}
			channel := int(buf1[0])
			rtpLen := int(binary.BigEndian.Uint16(buf2))
			rtpBytes := make([]byte, rtpLen)
			if _, err := io.ReadFull(session.connRW, rtpBytes); err != nil {
				log.Errorf("%s:%v", session, err)
				return
			}
			rtpBuf := bytes.NewBuffer(rtpBytes)
			var pack *RTPPack
			switch channel {
			case session.aRTPChannel[0]:
				pack = &RTPPack{
					Type:    RTP_TYPE_AUDIO,
					Buffer:  rtpBuf,
					Channel: 0,
				}
				elapsed := time.Now().Sub(timer)
				if elapsed >= 30*time.Second {
					log.Debugf("%s, Recv an audio RTP package", session.String())
					timer = time.Now()
				}
			case session.aRTPChannel[1]:
				pack = &RTPPack{
					Type:    RTP_TYPE_AUDIO,
					Buffer:  rtpBuf,
					Channel: 1,
				}
				elapsed := time.Now().Sub(timer)
				if elapsed >= 30*time.Second {
					log.Debugf("%s, Recv an audio RTP package", session.String())
					timer = time.Now()
				}
			case session.aRTPControlChannel[0]:
				pack = &RTPPack{
					Type:    RTP_TYPE_AUDIOCONTROL,
					Buffer:  rtpBuf,
					Channel: 0,
				}
			case session.aRTPControlChannel[1]:
				pack = &RTPPack{
					Type:    RTP_TYPE_AUDIOCONTROL,
					Buffer:  rtpBuf,
					Channel: 1,
				}
			case session.vRTPChannel:
				pack = &RTPPack{
					Type:   RTP_TYPE_VIDEO,
					Buffer: rtpBuf,
				}
				elapsed := time.Now().Sub(timer)
				if elapsed >= 30*time.Second {
					log.Debugf("[%s] Recv an video RTP package", session)
					timer = time.Now()
				}
			case session.vRTPControlChannel:
				pack = &RTPPack{
					Type:   RTP_TYPE_VIDEOCONTROL,
					Buffer: rtpBuf,
				}
			default:
				log.Errorf("[%s] unknow rtp pack type: %v", session, pack.Type)
				continue
			}
			if pack == nil {
				log.Errorf("[%s] get nil packet", session.String())
				continue
			}
			session.InBytes += uint(rtpLen) + 4
			for _, h := range session.RTPHandles {
				h(pack)
			}
		} else { // rtsp cmd
			reqBuf := bytes.NewBuffer(nil)
			reqBuf.Write(buf1)
			for !session.getStoped() {
				if line, isPrefix, err := session.connRW.ReadLine(); err != nil {
					log.Errorf("[%s]%v", session, err)
					return
				} else {
					reqBuf.Write(line)
					if !isPrefix {
						reqBuf.WriteString("\r\n")
					}
					if len(line) == 0 {
						req := NewRequest(reqBuf.String())
						if req == nil {
							break
						}
						session.InBytes += uint(reqBuf.Len())
						contentLen := req.GetContentLength()
						session.InBytes += uint(contentLen)
						if contentLen > 0 {
							bodyBuf := make([]byte, contentLen)
							if n, err := io.ReadFull(session.connRW, bodyBuf); err != nil {
								log.Errorf("[%s]%v", session, err)
								return
							} else if n != contentLen {
								log.Errorf("[%s] read rtsp request body failed, expect size[%d], got size[%d]",
									session, contentLen, n)
								return
							}
							req.Body = string(bodyBuf)
						}
						session.handleRequest(req)
						break
					}
				}
			}
		}
	}
}

func validate(key []byte, salt []byte, request string) string {
	mac := hmac.New(sha512.New, key)
	mac.Write(salt)
	signingKey := mac.Sum(nil)
	mac2 := hmac.New(sha512.New, signingKey)
	mac2.Write([]byte(request))
	return base64.StdEncoding.EncodeToString(mac2.Sum(nil))
}

func (session *Session) authenticate(req *Request) int {
	u, _ := url.ParseRequestURI(req.URL)
	exp := u.Query().Get("expires")
	salt := u.Query().Get("salt")
	signature := u.Query().Get("signature")
	if len(exp) == 0 || len(salt) == 0 || len(signature) == 0 {
		fmt.Printf("empty exp=%s, salt=%s or signature=%s", exp, salt, signature)
		return 401
	}
	params := strings.Split(u.RawQuery, "&")
	paramUrl := params[0]
	expTime, err := time.Parse("2006-01-02T15:04:05Z", exp)
	if err != nil {
		fmt.Printf("invalid exp=%s", exp)
		return 401
	}
	if time.Now().After(expTime) {
		fmt.Printf("signature has expired")
		return 403
	}
	buf := bytes.NewBufferString("TV")
	streamHex, _ := hex.DecodeString(session.streamSecret)
	buf.Write(streamHex)

	saltRaw, _ := base64.StdEncoding.DecodeString(salt)
	rawPath := strings.Split(req.URL, "?")[0]
	request := req.Method + "\n" + rawPath + "\n" + paramUrl
	if validate(buf.Bytes(), saltRaw, request) == signature {
		return 200
	} else {
		return 401
	}
}

func (session *Session) handleRequest(req *Request) {
	//if session.Timeout > 0 {
	//	session.Conn.SetDeadline(time.Now().Add(time.Duration(session.Timeout) * time.Second))
	//}
	log.Debugf("<<<\n%s", req)
	res := NewResponse(200, "OK", req.Header["CSeq"], session.ID, "")
	defer func() {
		if p := recover(); p != nil {
			log.Errorf("[%s] handleRequest err ocurs:%v", session, p)
			res.StatusCode = 500
			res.Status = fmt.Sprintf("Internal Server Error")
			debug.PrintStack()
		}
		log.Debugf(">>>\n%s", res)
		outBytes := []byte(res.String())
		session.connWLock.Lock()
		session.connRW.Write(outBytes)
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += uint(len(outBytes))
		switch req.Method {
		case "PLAY", "RECORD":
			switch session.Type {
			case SESSEION_TYPE_PLAYER:
				if session.Pusher.HasPlayer(session.Player) {
					session.Player.Pause(false)
				} else {
					session.Pusher.AddPlayer(session.Player)
				}
				// case SESSION_TYPE_PUSHER:
				// 	session.Server.AddPusher(session.Pusher)
			}
		case "TEARDOWN":
			{
				session.Stop()
				return
			}
		}
		if res.StatusCode != 200 && res.StatusCode != 401 {
			log.Errorf("Response request error[%d]. stop session.", res.StatusCode)
			session.Stop()
		}
	}()
	// Authorization part
	if req.Method != "OPTIONS" {
		if session.authorizationEnable {
			authLine := req.Header["Authorization"]
			authFailed := true
			if authLine != "" {
				err := CheckAuth(authLine, req.Method, session.nonce)
				if err == nil {
					authFailed = false
				} else {
					log.Errorf("[%s] %v", session, err)
				}
			}
			if authFailed {
				res.StatusCode = 401
				res.Status = "Unauthorized"
				nonce := fmt.Sprintf("%x", md5.Sum([]byte(shortid.MustGenerate())))
				session.nonce = nonce
				res.Header["WWW-Authenticate"] = fmt.Sprintf(`Digest realm="EasyDarwin", nonce="%s", algorithm="MD5"`, nonce)
				return
			}
		}
	}
	switch req.Method {
	case "OPTIONS":
		res.Header["Public"] = "DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE, OPTIONS, ANNOUNCE, RECORD"
	case "ANNOUNCE":
		session.Type = SESSION_TYPE_PUSHER
		session.URL = req.URL

		url, err := url.Parse(req.URL)
		if err != nil {
			res.StatusCode = 500
			res.Status = "Invalid URL"
			return
		}

		// This is to be consistent with API server.
		code := session.authenticate(req)
		if code != 200 {
			logger.Printf("auth status is not 200 %d", code)
			res.Status = "Unauthorized"
			res.StatusCode = code
			nonce := fmt.Sprintf("%x", md5.Sum([]byte(shortid.MustGenerate())))
			session.nonce = nonce
			res.Header["WWW-Authenticate"] = fmt.Sprintf(`Digest realm="EasyDarwin", nonce="%s", algorithm="MD5"`, nonce)
			return
		}
		session.Path = url.Path

		session.SDPRaw = req.Body
		session.Sdp, err = sdp.ParseString(req.Body)
		if err != nil {
			res.StatusCode = 400
			res.Status = "Bad Request"
		}

		for _, media := range session.Sdp.Media {
			switch media.Type {
			case "video":
				session.VControl = media.Attributes.Get("control")
				session.VCodec = media.Formats[0].Name
				log.Infof("video codec[%s]", session.VCodec)
			case "audio":
				session.AControl[session.aChannelNum] = media.Attributes.Get("control")
				session.ACodec[session.aChannelNum] = media.Formats[0].Name
				log.Infof("audio codec[%s]", session.ACodec[session.aChannelNum])
				session.aChannelNum++
			}
		}

		session.Pusher = NewPusher(session)

		addedToServer := session.Server.AddPusher(session.Pusher, session.closeOld)
		if !addedToServer {
			log.Infof("reject pusher[%s]", session.Pusher.ID())
			res.StatusCode = 406
			res.Status = "Not Acceptable"
		}
	case "DESCRIBE":
		session.Type = SESSEION_TYPE_PLAYER
		session.URL = req.URL

		url, err := url.Parse(req.URL)
		if err != nil {
			res.StatusCode = 500
			res.Status = "Invalid URL"
			return
		}
		// This is to be consistent with API server.
		code := session.authenticate(req)
		if code != 200 {
			logger.Printf("auth status is not 200 %d", code)
			res.Status = "Unauthorized"
			res.StatusCode = code
			nonce := fmt.Sprintf("%x", md5.Sum([]byte(shortid.MustGenerate())))
			session.nonce = nonce
			res.Header["WWW-Authenticate"] = fmt.Sprintf(`Digest realm="EasyDarwin", nonce="%s", algorithm="MD5"`, nonce)
			return
		}

		session.Path = url.Path
		pusher := session.Server.GetPusher(session.Path, session)
		if pusher == nil {
			res.StatusCode = 404
			res.Status = "NOT FOUND"
			return
		}
		session.Player = NewPlayer(session, pusher)
		session.Pusher = pusher
		session.AControl = pusher.AControl()
		session.VControl = pusher.VControl()
		session.ACodec = pusher.ACodec()
		session.VCodec = pusher.VCodec()
		session.Conn.timeout = 0
		res.SetBody(session.Pusher.SDPRaw())
	case "SETUP":
		ts := req.Header["Transport"]
		// control字段可能是`stream=1`字样，也可能是rtsp://...字样。即control可能是url的path，也可能是整个url
		// 例1：
		// a=control:streamid=1
		// 例2：
		// a=control:rtsp://192.168.1.64/trackID=1
		// 例3：
		// a=control:?ctype=video
		setupUrl, err := url.Parse(req.URL)
		if err != nil {
			res.StatusCode = 500
			res.Status = "Invalid URL"
			return
		}
		if setupUrl.Port() == "" {
			setupUrl.Host = fmt.Sprintf("%s:554", setupUrl.Host)
		}
		setupPath := setupUrl.String()

		// error status. SETUP without ANNOUNCE or DESCRIBE.
		if session.Pusher == nil {
			res.StatusCode = 500
			res.Status = "Error Status"
			return
		}
		//setupPath = setupPath[strings.LastIndex(setupPath, "/")+1:]
		vPath := ""
		if strings.Index(strings.ToLower(session.VControl), "rtsp://") == 0 {
			vControlUrl, err := url.Parse(session.VControl)
			if err != nil {
				res.StatusCode = 500
				res.Status = "Invalid VControl"
				return
			}
			if vControlUrl.Port() == "" {
				vControlUrl.Host = fmt.Sprintf("%s:554", vControlUrl.Host)
			}
			vPath = vControlUrl.String()
		} else {
			vPath = session.VControl
		}

		aPathes := []string{}
		for _, AControl := range session.AControl {
			if strings.Index(strings.ToLower(AControl), "rtsp://") == 0 {
				aControlURL, err := url.Parse(AControl)
				if err != nil {
					res.StatusCode = 500
					res.Status = "Invalid AControl"
					return
				}
				if aControlURL.Port() == "" {
					aControlURL.Host = fmt.Sprintf("%s:554", aControlURL.Host)
				}
				aPathes = append(aPathes, aControlURL.String())
			} else {
				aPathes = append(aPathes, AControl)
			}
		}

		mtcp := regexp.MustCompile("interleaved=(\\d+)(-(\\d+))?")
		mudp := regexp.MustCompile("client_port=(\\d+)(-(\\d+))?")

		if tcpMatchs := mtcp.FindStringSubmatch(ts); tcpMatchs != nil {
			session.TransType = TRANS_TYPE_TCP

			matchAudio := false
			for _, aPath := range aPathes {
				if setupPath == aPath || aPath != "" && strings.LastIndex(setupPath, aPath) == len(setupPath)-len(aPath) {
					session.aRTPChannel[session.aChannelNum], _ = strconv.Atoi(tcpMatchs[1])
					session.aRTPControlChannel[session.aChannelNum], _ = strconv.Atoi(tcpMatchs[3])
					session.aChannelNum++

					matchAudio = true
					break
				}
			}
			if matchAudio {
			} else if setupPath == vPath || vPath != "" && strings.LastIndex(setupPath, vPath) == len(setupPath)-len(vPath) {
				session.vRTPChannel, _ = strconv.Atoi(tcpMatchs[1])
				session.vRTPControlChannel, _ = strconv.Atoi(tcpMatchs[3])
			} else {
				res.StatusCode = 500
				res.Status = fmt.Sprintf("SETUP [TCP] got UnKown control:%s", setupPath)
				log.Errorf("SETUP [TCP] got UnKown control:%s", setupPath)
			}
			log.Infof("Parse SETUP req.TRANSPORT:TCP.Session.Type:%d,control:%s, AControl:%v,VControl:%s",
				session.Type, setupPath, aPathes, vPath)
		} else if udpMatchs := mudp.FindStringSubmatch(ts); udpMatchs != nil {
			session.TransType = TRANS_TYPE_UDP
			// no need for tcp timeout.
			session.Conn.timeout = 0
			if session.Type == SESSEION_TYPE_PLAYER && session.UDPClient == nil {
				session.UDPClient = NewUDPClient(session)
			}
			if session.Type == SESSION_TYPE_PUSHER && session.UDPServer == nil {
				session.UDPServer = NewUDPServerFromSession(session)
			}

			log.Infof("Parse SETUP req.TRANSPORT:UDP.Session.Type:%d,control:%s, AControl:%v,VControl:%s",
				session.Type, setupPath, aPathes, vPath)
			matchAudio := false
			for _, aPath := range aPathes {
				if setupPath == aPath || aPath != "" && strings.LastIndex(setupPath, aPath) == len(setupPath)-len(aPath) {
					if session.Type == SESSEION_TYPE_PLAYER {
						session.UDPClient.APort[session.aChannelNum], _ = strconv.Atoi(udpMatchs[1])
						session.UDPClient.AControlPort[session.aChannelNum], _ = strconv.Atoi(udpMatchs[3])
						if err := session.UDPClient.SetupAudio(session.aChannelNum); err != nil {
							res.StatusCode = 500
							res.Status = fmt.Sprintf("udp client setup audio error, %v", err)
							return
						}
						session.aChannelNum++
					} else if session.Type == SESSION_TYPE_PUSHER {
						if err := session.UDPServer.SetupAudio(session.aChannelNum); err != nil {
							res.StatusCode = 500
							res.Status = fmt.Sprintf("udp server setup audio error, %v", err)
							return
						}
						session.aChannelNum++
						tss := strings.Split(ts, ";")
						idx := -1
						for i, val := range tss {
							if val == udpMatchs[0] {
								idx = i
							}
						}
						tail := append([]string{}, tss[idx+1:]...)
						tss = append(tss[:idx+1], fmt.Sprintf("server_port=%d-%d", session.UDPServer.APort, session.UDPServer.AControlPort))
						tss = append(tss, tail...)
						ts = strings.Join(tss, ";")
					} else {
						log.Errorf("Unknow session type:%d", session.Type)
						res.StatusCode = 400
						res.Status = "Bad Request"
						return
					}
				}
			}
			if matchAudio {

			} else if setupPath == vPath || vPath != "" && strings.LastIndex(setupPath, vPath) == len(setupPath)-len(vPath) {
				if session.Type == SESSEION_TYPE_PLAYER {
					session.UDPClient.VPort, _ = strconv.Atoi(udpMatchs[1])
					session.UDPClient.VControlPort, _ = strconv.Atoi(udpMatchs[3])
					if err := session.UDPClient.SetupVideo(); err != nil {
						res.StatusCode = 500
						res.Status = fmt.Sprintf("udp client setup video error, %v", err)
						return
					}
				}

				if session.Type == SESSION_TYPE_PUSHER {
					if err := session.UDPServer.SetupVideo(); err != nil {
						res.StatusCode = 500
						res.Status = fmt.Sprintf("udp server setup video error, %v", err)
						return
					}
					tss := strings.Split(ts, ";")
					idx := -1
					for i, val := range tss {
						if val == udpMatchs[0] {
							idx = i
						}
					}
					tail := append([]string{}, tss[idx+1:]...)
					tss = append(tss[:idx+1], fmt.Sprintf("server_port=%d-%d", session.UDPServer.VPort, session.UDPServer.VControlPort))
					tss = append(tss, tail...)
					ts = strings.Join(tss, ";")
				}
			} else {
				log.Errorf("SETUP [UDP] got UnKown control:%s", setupPath)
			}
		}
		res.Header["Transport"] = ts
	case "PLAY":
		// error status. PLAY without ANNOUNCE or DESCRIBE.
		if session.Pusher == nil {
			res.StatusCode = 500
			res.Status = "Error Status"
			return
		}
		res.Header["Range"] = req.Header["Range"]
	case "RECORD":
		// error status. RECORD without ANNOUNCE or DESCRIBE.
		if session.Pusher == nil {
			res.StatusCode = 500
			res.Status = "Error Status"
			return
		}
	case "PAUSE":
		if session.Player == nil {
			res.StatusCode = 500
			res.Status = "Error Status"
			return
		}
		session.Player.Pause(true)
	}
}

func (session *Session) SendRTP(pack *RTPPack) (err error) {
	if pack == nil {
		err = fmt.Errorf("player send rtp got nil pack")
		return
	}
	if session.TransType == TRANS_TYPE_UDP {
		if session.UDPClient == nil {
			err = fmt.Errorf("player use udp transport but udp client not found")
			return
		}
		err = session.UDPClient.SendRTP(pack)
		return
	}
	switch pack.Type {
	case RTP_TYPE_AUDIO:
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(session.aRTPChannel[pack.Channel])
		session.connWLock.Lock()
		session.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(pack.Buffer.Len()))
		session.connRW.Write(bufLen)
		session.connRW.Write(pack.Buffer.Bytes())
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += uint(pack.Buffer.Len()) + 4
	case RTP_TYPE_AUDIOCONTROL:
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(session.aRTPControlChannel[pack.Channel])
		session.connWLock.Lock()
		session.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(pack.Buffer.Len()))
		session.connRW.Write(bufLen)
		session.connRW.Write(pack.Buffer.Bytes())
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += uint(pack.Buffer.Len()) + 4
	case RTP_TYPE_VIDEO:
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(session.vRTPChannel)
		session.connWLock.Lock()
		session.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(pack.Buffer.Len()))
		session.connRW.Write(bufLen)
		session.connRW.Write(pack.Buffer.Bytes())
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += uint(pack.Buffer.Len()) + 4
	case RTP_TYPE_VIDEOCONTROL:
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(session.vRTPControlChannel)
		session.connWLock.Lock()
		session.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(pack.Buffer.Len()))
		session.connRW.Write(bufLen)
		session.connRW.Write(pack.Buffer.Bytes())
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += uint(pack.Buffer.Len()) + 4
	default:
		err = fmt.Errorf("session tcp send rtp got unkown pack type[%v]", pack.Type)
	}
	return
}

func (session *Session) getStoped() bool {
	session.stopedLock.RLock()
	isStop := session.Stoped
	session.stopedLock.RUnlock()
	return isStop
}

func (session *Session) setStoped(stop bool) {
	session.stopedLock.Lock()
	session.Stoped = stop
	session.stopedLock.Unlock()
	return
}