package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/EasyDarwin/EasyDarwin/models"
	"github.com/EasyDarwin/EasyDarwin/routers"
	"github.com/EasyDarwin/EasyDarwin/rtsp"
	"github.com/EasyDarwin/EasyDarwin/utils"
	"github.com/common-nighthawk/go-figure"
	"github.com/penggy/EasyGoLib/db"
	"github.com/penggy/service"
)

var (
	gitCommitCode string
	buildDateTime string
)

type program struct {
	httpPort     int
	httpServer   *http.Server
	rtspPort     int
	rtspServer   *rtsp.Server
	cert         string
	key          string
	streamSecret string
}

func (p *program) StopHTTP() (err error) {
	if p.httpServer == nil {
		err = fmt.Errorf("HTTP Server Not Found")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = p.httpServer.Shutdown(ctx); err != nil {
		return
	}
	return
}

func (p *program) StartHTTP() (err error) {

	p.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", p.httpPort),
		Handler:           routers.Router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	link := fmt.Sprintf("https://%s:%d", utils.LocalIP(), p.httpPort)
	log.Println("https server start -->", link)
	go func() {
		if err := p.httpServer.ListenAndServeTLS(p.cert, p.key); err != nil && err != http.ErrServerClosed {
			log.Println("start http server error", err)
		}
		log.Println("https server end")
	}()
	return
}

func (p *program) StartRTSP() (err error) {
	if p.rtspServer == nil {
		err = fmt.Errorf("RTSP Server Not Found")
		return
	}
	sport := ""
	if p.rtspPort != 554 {
		sport = fmt.Sprintf(":%d", p.rtspPort)
	}
	link := fmt.Sprintf("rtsps://%s%s", utils.LocalIP(), sport)
	log.Println("rtsp server start -->", link)
	go func() {
		if err := p.rtspServer.Start(p.cert, p.key, p.streamSecret); err != nil {
			log.Println("start rtsp server error", err)
		}
		log.Println("rtsp server end")
	}()
	return
}

func (p *program) StopRTSP() (err error) {
	if p.rtspServer == nil {
		err = fmt.Errorf("RTSP Server Not Found")
		return
	}
	p.rtspServer.Stop()
	return
}

func (p *program) Start(s service.Service) (err error) {
	log.Println("********** START **********")
	if utils.IsPortInUse(p.httpPort) {
		err = fmt.Errorf("HTTP port[%d] In Use", p.httpPort)
		return
	}
	if utils.IsPortInUse(p.rtspPort) {
		err = fmt.Errorf("RTSP port[%d] In Use", p.rtspPort)
		return
	}
	// Init API server
	err = routers.Init()
	if err != nil {
		return
	}
	p.StartRTSP()
	p.StartHTTP()

	// TODO: log sestup
	go func() {
		for range routers.API.RestartChan {
			p.StopHTTP()
			p.StopRTSP()
			// utils.ReloadConf()
			// TODO : reload config and init
			p.StartRTSP()
			p.StartHTTP()
		}
	}()

	// Restart pushers
	go func() {
		log.Printf("demon pull streams")
		for {
			var streams []*models.Stream
			streams, err := models.GetAllStream()
			if err != nil {
				log.Printf("find stream err:%v", err)
				time.Sleep(10 * time.Second)
				continue
			}
			for _, v := range streams {
				if nil != rtsp.Instance.GetPusher(v.CustomPath, nil) {
					continue
				}

				agent := fmt.Sprintf("EasyDarwinGo/%s", routers.BuildVersion)
				if routers.BuildDateTime != "" {
					agent = fmt.Sprintf("%s(%s)", agent, routers.BuildDateTime)
				}
				client, err := rtsp.NewRTSPClient(
					rtsp.GetServer(), v.ID, v.URL, int64(v.HeartbeatInterval)*1000, agent)
				if err != nil {
					continue
				}
				client.CustomPath = v.CustomPath

				pusher := rtsp.NewClientPusher(client)
				err = client.Start(time.Duration(v.IdleTimeout) * time.Second)
				if err != nil {
					log.Printf("Pull stream err :%v", err)
					continue
				}
				rtsp.GetServer().AddPusher(pusher)
				//streams = streams[0:i]
				//streams = append(streams[:i], streams[i+1:]...)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	// Restart record
	go func() {
		log.Printf("demon pull recorders")
		for {
			tasks := models.GetAllTasks()
			for _, task := range tasks {
				log.Print(task.String())
				// TODO: More safe pusher.AddPlayer
				pusher := rtsp.Instance.GetPusher(task.PlayPath, nil)
				if nil == pusher {
					continue
				}
				if nil != pusher.GetPlayer(task.ID) {
					continue
				}
				log.Printf("Task[%s] off, restarting", task.ID)
				recorder, err := rtsp.NewRecorder(task, pusher)
				if nil != err {
					log.Printf("NewRecorder error[%v]", err)
					continue
				}
				if err = pusher.AddPlayer(recorder); nil != err {
					log.Printf("Addplayer error[%v]", err)
					continue
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()
	return
}

func (p *program) Stop(s service.Service) (err error) {
	defer log.Println("********** STOP **********")
	// defer utils.CloseLogWriter()
	// TODO: stop log
	p.StopHTTP()
	p.StopRTSP()
	return
}

func main() {
	// TODO: input config filepathf
	// flag.StringVar(&utils.FlagVarConfFile, "config", "", "configure file path")
	flag.Parse()
	tail := flag.Args()

	// log
	log.SetOutput(os.Stdout)
	log.SetPrefix("[EasyDarwin] ")
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	log.Printf("git commit code:%s", gitCommitCode)
	log.Printf("build date:%s", buildDateTime)
	routers.BuildVersion = fmt.Sprintf("%s.%s", routers.BuildVersion, gitCommitCode)
	routers.BuildDateTime = buildDateTime

	svcConfig := &service.Config{
		Name:        config.Service.Name,
		DisplayName: config.Service.DisplayName,
		Description: config.Service.Description,
	}

	httpPort := config.HTTP.Port
	cert := utils.Conf().Section("tls").Key("cert").MustString("")
	key := utils.Conf().Section("tls").Key("key").MustString("")
	streamSecret := utils.Conf().Section("rtsp").Key("stream_secret_key").MustString("")
	rtspServer := rtsp.GetServer()
	p := &program{
		httpPort:     httpPort,
		rtspPort:     rtspServer.TCPPort,
		rtspServer:   rtspServer,
		cert:         cert,
		key:          key,
		streamSecret: streamSecret,
	}
	s, err := service.New(p, svcConfig)
	if err != nil {
		log.Println(err)
		utils.PauseExit()
	}
	if len(tail) > 0 {
		cmd := strings.ToLower(tail[0])
		if cmd == "install" || cmd == "stop" || cmd == "start" || cmd == "uninstall" {
			figure.NewFigure("EasyDarwin", "", false).Print()
			log.Println(svcConfig.Name, cmd, "...")
			if err = service.Control(s, cmd); err != nil {
				log.Println(err)
				utils.PauseExit()
			}
			log.Println(svcConfig.Name, cmd, "ok")
			return
		}
	}
	figure.NewFigure("EasyDarwin", "", false).Print()
	if err = s.Run(); err != nil {
		log.Println(err)
		utils.PauseExit()
	}
}
