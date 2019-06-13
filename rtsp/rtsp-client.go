package rtsp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pixelbender/go-sdp/sdp"
)

type RTSPClient struct {
	Server               *Server
	Stoped               bool
	Status               string
	URL                  string
	Path                 string
	CustomPath           string //custom path for pusher
	ID                   string
	Conn                 *RichConn
	Session              string
	Seq                  int
	connRW               *bufio.ReadWriter
	InBytes              uint
	OutBytes             uint
	TransType            TransType
	StartAt              time.Time
	Sdp                  *sdp.Session
	AControl             []string
	VControl             string
	ACodec               []string
	VCodec               string
	OptionIntervalMillis int64
	SDPRaw               string

	Agent    string
	authLine string

	//tcp channels
	aRTPChannel        []int
	aRTPControlChannel []int
	vRTPChannel        int
	vRTPControlChannel int

	// audio double channel
	aChannelNum int

	UDPServer   *UDPServer
	RTPHandles  []func(*RTPPack)
	StopHandles []func()
}

func (client *RTSPClient) String() string {
	return fmt.Sprintf("client[%s]", client.URL)
}

// NewRTSPClient for pull streams
func NewRTSPClient(
	server *Server,
	ID string,
	rawURL string,
	sendOptionMillis int64,
	agent string) (client *RTSPClient, err error) {

	_url, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	client = &RTSPClient{
		Server:               server,
		Stoped:               false,
		URL:                  rawURL,
		ID:                   ID,
		Path:                 _url.Path,
		TransType:            TRANS_TYPE_TCP,
		vRTPChannel:          0,
		vRTPControlChannel:   1,
		AControl:             []string{"not set up audio 01", "not set up audio 02"},
		ACodec:               []string{"invalid codec", "invalid codec"},
		aRTPChannel:          []int{2, 4},
		aRTPControlChannel:   []int{3, 5},
		OptionIntervalMillis: sendOptionMillis,
		StartAt:              time.Now(),
		Agent:                agent,
	}

	return
}

func (client *RTSPClient) requestStream(timeout time.Duration) (err error) {
	defer func() {
		if err != nil {
			client.Status = "Error"
		} else {
			client.Status = "OK"
		}
	}()
	l, err := url.Parse(client.URL)
	if err != nil {
		return err
	}
	if strings.ToLower(l.Scheme) != "rtsp" {
		err = fmt.Errorf("RTSP url is invalid")
		return err
	}
	if strings.ToLower(l.Hostname()) == "" {
		err = fmt.Errorf("RTSP url is invalid")
		return err
	}
	port := l.Port()
	if len(port) == 0 {
		port = "554"
	}
	conn, err := net.DialTimeout("tcp", l.Hostname()+":"+port, timeout)
	if err != nil {
		// handle error
		return err
	}

	networkBuffer := config.RTSP.NetworkBuffer

	timeoutConn := RichConn{
		conn,
		timeout,
	}
	client.Conn = &timeoutConn
	client.connRW = bufio.NewReadWriter(bufio.NewReaderSize(&timeoutConn, networkBuffer), bufio.NewWriterSize(&timeoutConn, networkBuffer))

	headers := make(map[string]string)
	headers["Require"] = "implicit-play"
	// An OPTIONS request returns the request types the server will accept.
	resp, err := client.Request("OPTIONS", headers)
	if err != nil {
		if resp != nil {
			Authorization, err := client.checkAuth("OPTIONS", resp)
			if err != nil {
				return err
			}
			if len(Authorization) > 0 {
				headers := make(map[string]string)
				headers["Require"] = "implicit-play"
				headers["Authorization"] = Authorization
				// An OPTIONS request returns the request types the server will accept.
				resp, err = client.Request("OPTIONS", headers)
			}
		} else {
			return err
		}
	}

	// A DESCRIBE request includes an RTSP URL (rtsp://...), and the type of reply data that can be handled. This reply includes the presentation description,
	// typically in Session Description Protocol (SDP) format. Among other things, the presentation description lists the media streams controlled with the aggregate URL.
	// In the typical case, there is one media stream each for audio and video.
	headers = make(map[string]string)
	headers["Accept"] = "application/sdp"
	resp, err = client.Request("DESCRIBE", headers)
	if err != nil {
		if resp != nil {
			authorization, _ := client.checkAuth("DESCRIBE", resp)
			if len(authorization) > 0 {
				headers := make(map[string]string)
				headers["Authorization"] = authorization
				headers["Accept"] = "application/sdp"
				resp, err = client.Request("DESCRIBE", headers)
			}
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	_sdp, err := sdp.ParseString(resp.Body)
	if err != nil {
		return err
	}
	client.Sdp = _sdp
	client.SDPRaw = resp.Body
	session := ""
	for _, media := range _sdp.Media {
		switch media.Type {
		case "video":
			client.VControl = media.Attributes.Get("control")
			client.VCodec = media.Formats[0].Name
			var _url = ""
			if strings.Index(strings.ToLower(client.VControl), "rtsp://") == 0 {
				_url = client.VControl
			} else {
				_url = strings.TrimRight(client.URL, "/") + "/" + strings.TrimLeft(client.VControl, "/")
			}
			headers = make(map[string]string)
			if client.TransType == TRANS_TYPE_TCP {
				headers["Transport"] = fmt.Sprintf("RTP/AVP/TCP;unicast;interleaved=%d-%d", client.vRTPChannel, client.vRTPControlChannel)
			} else {
				if client.UDPServer == nil {
					client.UDPServer = NewUDPServerFromClient(client)
				}
				//RTP/AVP;unicast;client_port=64864-64865
				err = client.UDPServer.SetupVideo()
				if err != nil {
					log.Error("Setup video err.%v", err)
					return err
				}
				headers["Transport"] = fmt.Sprintf("RTP/AVP/UDP;unicast;client_port=%d-%d", client.UDPServer.VPort, client.UDPServer.VControlPort)
				client.Conn.timeout = 0 //	UDP ignore timeout
			}
			if session != "" {
				headers["Session"] = session
			}
			log.Info("Parse DESCRIBE response, VIDEO VControl:%s, VCode:%s, url:%s,Session:%s,vRTPChannel:%d,vRTPControlChannel:%d",
				client.VControl, client.VCodec, _url, session, client.vRTPChannel, client.vRTPControlChannel)
			resp, err = client.RequestWithPath("SETUP", _url, headers, true)
			if err != nil {
				return err
			}
			session, _ = resp.Header["Session"].(string)
		case "audio":
			if client.aChannelNum >= 2 {
				log.Error("Session[%s] more than 2 channel, please look into it", session)
				continue
			}
			log.Infof("Session[%s] Setup audio channel:%d", session, client.aChannelNum)
			client.AControl[client.aChannelNum] = media.Attributes.Get("control")
			client.ACodec[client.aChannelNum] = media.Formats[0].Name
			AControl := client.AControl[client.aChannelNum]
			// ACodec := client.ACodec[client.aChannelNum]
			var _url = ""
			if strings.Index(strings.ToLower(AControl), "rtsp://") == 0 {
				_url = AControl
			} else {
				_url = strings.TrimRight(client.URL, "/") + "/" + strings.TrimLeft(AControl, "/")
			}
			headers = make(map[string]string)
			if client.TransType == TRANS_TYPE_TCP {
				headers["Transport"] = fmt.Sprintf(
					"RTP/AVP/TCP;unicast;interleaved=%d-%d",
					client.aRTPChannel[client.aChannelNum],
					client.aRTPControlChannel[client.aChannelNum])
			} else {
				if client.UDPServer == nil {
					client.UDPServer = NewUDPServerFromClient(client)
				}
				err = client.UDPServer.SetupAudio(client.aChannelNum)
				if err != nil {
					log.Errorf("Session[%s] Setup audio err.%v", session, err)
					return err
				}
				headers["Transport"] = fmt.Sprintf("RTP/AVP/UDP;unicast;client_port=%d-%d", client.UDPServer.APort, client.UDPServer.AControlPort)
				client.Conn.timeout = 0 //	UDP ignore timeout
			}
			if session != "" {
				headers["Session"] = session
			}
			log.Infof("Parse DESCRIBE response, AUDIO AControl:%s, ACodec:%s, url:%s,Session:%s, aRTPChannel:%d,aRTPControlChannel:%d",
				client.AControl, client.ACodec, _url, session, client.aRTPChannel, client.aRTPControlChannel)
			resp, err = client.RequestWithPath("SETUP", _url, headers, true)
			if err != nil {
				return err
			}
			session, _ = resp.Header["Session"].(string)
			// Setup success
			client.aChannelNum++
		}
	}
	headers = make(map[string]string)
	if session != "" {
		headers["Session"] = session
	}
	resp, err = client.Request("PLAY", headers)
	if err != nil {
		return err
	}
	return nil
}

func (client *RTSPClient) startStream() {
	defer client.Stop()
	
	startTime := time.Now()
	loggerTime := time.Now().Add(-10 * time.Second)
	for !client.Stoped {
		if client.OptionIntervalMillis > 0 {
			if time.Since(startTime) > time.Duration(client.OptionIntervalMillis)*time.Millisecond {
				startTime = time.Now()
				headers := make(map[string]string)
				headers["Require"] = "implicit-play"
				// An OPTIONS request returns the request types the server will accept.
				if err := client.RequestNoResp("OPTIONS", headers); err != nil {
					// ignore...
				}
			}
		}
		b, err := client.connRW.ReadByte()
		if err != nil {
			if !client.Stoped {
				log.Infof("client.connRW.ReadByte err:%v", err)
			}
			return
		}
		switch b {
		case 0x24: // rtp
			header := make([]byte, 4)
			header[0] = b
			_, err := io.ReadFull(client.connRW, header[1:])
			if err != nil {

				if !client.Stoped {
					log.Infof("io.ReadFull err:%v", err)
				}
				return
			}
			channel := int(header[1])
			length := binary.BigEndian.Uint16(header[2:])
			content := make([]byte, length)
			_, err = io.ReadFull(client.connRW, content)
			if err != nil {
				if !client.Stoped {
					log.Infof("io.ReadFull err:%v", err)
				}
				return
			}
			//ch <- append(header, content...)
			rtpBuf := bytes.NewBuffer(content)
			var pack *RTPPack
			switch channel {
			case client.aRTPChannel[0]:
				pack = &RTPPack{
					Type:    RTP_TYPE_AUDIO,
					Buffer:  rtpBuf,
					Channel: 0,
				}
			case client.aRTPChannel[1]:
				pack = &RTPPack{
					Type:    RTP_TYPE_AUDIO,
					Buffer:  rtpBuf,
					Channel: 1,
				}
			case client.aRTPControlChannel[0]:
				pack = &RTPPack{
					Type:    RTP_TYPE_AUDIOCONTROL,
					Buffer:  rtpBuf,
					Channel: 0,
				}
			case client.aRTPControlChannel[1]:
				pack = &RTPPack{
					Type:    RTP_TYPE_AUDIOCONTROL,
					Buffer:  rtpBuf,
					Channel: 1,
				}
			case client.vRTPChannel:
				pack = &RTPPack{
					Type:   RTP_TYPE_VIDEO,
					Buffer: rtpBuf,
				}
			case client.vRTPControlChannel:
				pack = &RTPPack{
					Type:   RTP_TYPE_VIDEOCONTROL,
					Buffer: rtpBuf,
				}
			default:
				log.Errorf("unknow rtp pack type, channel:%v", channel)
				continue
			}

			elapsed := time.Now().Sub(loggerTime)
			if elapsed >= 10*time.Second {
				log.Debugf("%v read rtp frame.", client)
				loggerTime = time.Now()
			}
			client.InBytes += uint(length) + 4
			for _, h := range client.RTPHandles {
				h(pack)
			}

		default: // rtsp
			builder := bytes.Buffer{}
			builder.WriteByte(b)
			contentLen := 0
			for !client.Stoped {
				line, prefix, err := client.connRW.ReadLine()
				if err != nil {
					if !client.Stoped {
						log.Infof("client.connRW.ReadLine err:%v", err)
					}
					return
				}
				if len(line) == 0 {
					if contentLen != 0 {
						content := make([]byte, contentLen)
						_, err = io.ReadFull(client.connRW, content)
						if err != nil {
							if !client.Stoped {
								err = fmt.Errorf("Read content err.ContentLength:%d", contentLen)
							}
							return
						}
						builder.Write(content)
					}
					log.Debugf("<<<[IN]\n%s", builder.String())
					break
				}
				s := string(line)
				builder.Write(line)
				if !prefix {
					builder.WriteString("\r\n")
				}

				if strings.Index(s, "Content-Length:") == 0 {
					splits := strings.Split(s, ":")
					contentLen, err = strconv.Atoi(strings.TrimSpace(splits[1]))
					if err != nil {
						if !client.Stoped {
							log.Errorf("strconv.Atoi err:%v, str:%v", err, splits[1])
						}
						return
					}
				}
			}
		}
	}
}

func (client *RTSPClient) Start(timeout time.Duration) (err error) {
	if timeout == 0 {
		timeoutMillis := config.RTSP.Timeout
		timeout = time.Duration(timeoutMillis) * time.Millisecond
	}
	err = client.requestStream(timeout)
	if err != nil {
		return
	}
	go client.startStream()
	return
}

func (client *RTSPClient) Stop() {
	if client.Stoped {
		return
	}
	client.Stoped = true
	for _, h := range client.StopHandles {
		h()
	}
	if client.Conn != nil {
		client.connRW.Flush()
		client.Conn.Close()
		client.Conn = nil
	}
	if client.UDPServer != nil {
		client.UDPServer.Stop()
		client.UDPServer = nil
	}
}

func (client *RTSPClient) RequestWithPath(method string, path string, headers map[string]string, needResp bool) (resp *Response, err error) {
	headers["User-Agent"] = client.Agent
	if len(headers["Authorization"]) == 0 {
		if len(client.authLine) != 0 {
			Authorization, _ := DigestAuth(client.authLine, method, client.URL)
			if len(Authorization) > 0 {
				headers["Authorization"] = Authorization
			}
		}
	}
	if len(client.Session) > 0 {
		headers["Session"] = client.Session
	}
	client.Seq++
	cseq := client.Seq
	builder := bytes.Buffer{}
	builder.WriteString(fmt.Sprintf("%s %s RTSP/1.0\r\n", method, path))
	builder.WriteString(fmt.Sprintf("CSeq: %d\r\n", cseq))
	for k, v := range headers {
		builder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	builder.WriteString(fmt.Sprintf("\r\n"))
	s := builder.String()
	log.Debugf("[OUT]>>>\n%s", s)
	_, err = client.connRW.WriteString(s)
	if err != nil {
		return
	}
	client.connRW.Flush()

	if !needResp {
		return nil, nil
	}
	lineCount := 0
	statusCode := 200
	status := ""
	sid := ""
	contentLen := 0
	respHeader := make(map[string]interface{})
	var line []byte
	builder.Reset()
	for !client.Stoped {
		isPrefix := false
		if line, isPrefix, err = client.connRW.ReadLine(); err != nil {
			return
		}
		s := string(line)
		builder.Write(line)
		if !isPrefix {
			builder.WriteString("\r\n")
		}
		if len(line) == 0 {
			body := ""
			if contentLen > 0 {
				content := make([]byte, contentLen)
				_, err = io.ReadFull(client.connRW, content)
				if err != nil {
					err = fmt.Errorf("Read content err.ContentLength:%d", contentLen)
					return
				}
				body = string(content)
				builder.Write(content)
			}
			resp = NewResponse(statusCode, status, strconv.Itoa(cseq), sid, body)
			resp.Header = respHeader
			log.Debugf("<<<[IN]\n%s", builder.String())

			if !(statusCode >= 200 && statusCode <= 300) {
				err = fmt.Errorf("Response StatusCode is :%d", statusCode)
				return
			}
			return
		}
		if lineCount == 0 {
			splits := strings.Split(s, " ")
			if len(splits) < 3 {
				err = fmt.Errorf("StatusCode Line error:%s", s)
				return
			}
			statusCode, err = strconv.Atoi(splits[1])
			if err != nil {
				return
			}
			status = splits[2]
		}
		lineCount++
		splits := strings.Split(s, ":")
		if len(splits) == 2 {
			if val, ok := respHeader[splits[0]]; ok {
				if slice, ok2 := val.([]string); ok2 {
					slice = append(slice, strings.TrimSpace(splits[1]))
					respHeader[splits[0]] = slice
				} else {
					str, _ := val.(string)
					slice := []string{str, strings.TrimSpace(splits[1])}
					respHeader[splits[0]] = slice
				}
			} else {
				respHeader[splits[0]] = strings.TrimSpace(splits[1])
			}
		}
		if strings.Index(s, "Session:") == 0 {
			splits := strings.Split(s, ":")
			sid = strings.TrimSpace(splits[1])
		}
		//if strings.Index(s, "CSeq:") == 0 {
		//	splits := strings.Split(s, ":")
		//	cseq, err = strconv.Atoi(strings.TrimSpace(splits[1]))
		//	if err != nil {
		//		err = fmt.Errorf("Atoi CSeq err. line:%s", s)
		//		return
		//	}
		//}
		if strings.Index(s, "Content-Length:") == 0 {
			splits := strings.Split(s, ":")
			contentLen, err = strconv.Atoi(strings.TrimSpace(splits[1]))
			if err != nil {
				return
			}
		}

	}
	if client.Stoped {
		err = fmt.Errorf("Client Stoped.")
	}
	return
}

func (client *RTSPClient) Request(method string, headers map[string]string) (*Response, error) {
	l, err := url.Parse(client.URL)
	if err != nil {
		return nil, fmt.Errorf("Url parse error:%v", err)
	}
	l.User = nil
	return client.RequestWithPath(method, l.String(), headers, true)
}

func (client *RTSPClient) RequestNoResp(method string, headers map[string]string) (err error) {
	l, err := url.Parse(client.URL)
	if err != nil {
		return fmt.Errorf("Url parse error:%v", err)
	}
	l.User = nil
	if _, err = client.RequestWithPath(method, l.String(), headers, false); err != nil {
		return err
	}
	return nil
}
