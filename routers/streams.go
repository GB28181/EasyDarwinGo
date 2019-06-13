package routers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/EasyDarwin/EasyDarwin/models"
	"github.com/EasyDarwin/EasyDarwin/rtsp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

/**
 * @apiDefine stream 流管理
 */

/**
 * @api {get} /api/v1/stream/start 启动拉转推
 * @apiGroup stream
 * @apiName StreamStart
 * @apiParam {String} url RTSP源地址
 * @apiParam {String} [customPath] 转推时的推送PATH
 * @apiParam {String=TCP,UDP} [transType=TCP] 拉流传输模式
 * @apiParam {Number} [idleTimeout] 拉流时的超时时间
 * @apiParam {Number} [heartbeatInterval] 拉流时的心跳间隔，毫秒为单位。如果心跳间隔不为0，那拉流时会向源地址以该间隔发送OPTION请求用来心跳保活
 * @apiSuccess (200) {String} ID	拉流的ID。后续可以通过该ID来停止拉流
 */
func (h *APIHandler) StreamStart(c *gin.Context) {
	type Form struct {
		URL               string `form:"url" binding:"required"`
		CustomPath        string `form:"customPath"`
		TransType         string `form:"transType"`
		IdleTimeout       int64  `form:"idleTimeout"`
		HeartbeatInterval int64  `form:"heartbeatInterval"`
	}
	var form Form
	err := c.Bind(&form)
	if err != nil {
		log.Printf("Pull to push err:%v", err)
		return
	}
	agent := fmt.Sprintf("EasyDarwinGo/%s", BuildVersion)
	if BuildDateTime != "" {
		agent = fmt.Sprintf("%s(%s)", agent, BuildDateTime)
	}
	client, err := rtsp.NewRTSPClient(
		rtsp.GetServer(),
		uuid.New().String(),
		form.URL,
		int64(form.HeartbeatInterval)*1000,
		agent)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err.Error())
		return
	}
	if form.CustomPath != "" && !strings.HasPrefix(form.CustomPath, "/") {
		form.CustomPath = "/" + form.CustomPath
	}
	client.CustomPath = form.CustomPath
	switch strings.ToLower(form.TransType) {
	case "udp":
		client.TransType = rtsp.TRANS_TYPE_UDP
	case "tcp":
		fallthrough
	default:
		client.TransType = rtsp.TRANS_TYPE_TCP
	}

	pusher := rtsp.NewClientPusher(client)
	if rtsp.GetServer().GetPusher(pusher.Path(), nil) != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("Path %s already exists", client.Path))
		return
	}
	err = client.Start(time.Duration(form.IdleTimeout) * time.Second)
	if err != nil {
		log.Printf("Pull stream err :%v", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("Pull stream err: %v", err))
		return
	}
	log.Printf("Pull to push %v success ", form)
	rtsp.GetServer().AddPusher(pusher, false)
	// save to db.
	stream := models.Stream{
		ID:                client.ID,
		URL:               form.URL,
		CustomPath:        form.CustomPath,
		IdleTimeout:       form.IdleTimeout,
		HeartbeatInterval: form.HeartbeatInterval,
	}

	err = models.AddStream(&stream)
	if err != nil {
		pusher.Stop()
		log.Printf("Pull stream err :%v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("Pull stream err: %v", err))
		return
	}

	c.IndentedJSON(200, pusher.ID())
}

/**
 * @api {get} /api/v1/stream/stop 停止推流
 * @apiGroup stream
 * @apiName StreamStop
 * @apiParam {String} id 拉流的ID
 * @apiUse simpleSuccess
 */
func (h *APIHandler) StreamStop(c *gin.Context) {
	type Form struct {
		ID string `form:"id" binding:"required"`
	}
	var form Form
	err := c.Bind(&form)
	if err != nil {
		log.Printf("stop pull to push err:%v", err)
		return
	}
	pushers := rtsp.GetServer().GetPushers()
	for it := pushers.Iterator(); !it.Done(); {
		_, _pusher := it.Next()
		pusher, ok := _pusher.(rtsp.Pusher)
		if ok && pusher.ID() == form.ID {
			// Remove first, in case of restart according to DB
			pusher.Server().RemovePusher(pusher.Path())
			if pusher.Mode() == rtsp.PusherModePull {
				models.RemoveStream(pusher.ID())
			}
			// TODO: forbidden ID to restart in a while,
			// or pause restart , remove from DB and stop it.
			c.IndentedJSON(200, "OK")
			log.Printf("Stop %s success ", pusher.ID())
			return

		}
	}
	c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("Pusher[%s] not found", form.ID))
}
