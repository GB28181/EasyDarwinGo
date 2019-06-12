package routers

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/EasyDarwin/EasyDarwin/rtsp"
	"github.com/EasyDarwin/EasyDarwin/utils"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

/**
 * @apiDefine sys 系统
 */

type APIHandler struct {
	RestartChan chan bool
}

var API = &APIHandler{
	RestartChan: make(chan bool),
}

var (
	memData    []PercentData = make([]PercentData, 0)
	cpuData    []PercentData = make([]PercentData, 0)
	pusherData []CountData   = make([]CountData, 0)
	playerData []CountData   = make([]CountData, 0)
)

func init() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		timeSize := 30
		for {
			select {
			case <-ticker.C:
				mem, _ := mem.VirtualMemory()
				cpu, _ := cpu.Percent(0, false)
				now := utils.DateTime(time.Now())
				memData = append(memData, PercentData{Time: now, Used: mem.UsedPercent / 100})
				cpuData = append(cpuData, PercentData{Time: now, Used: cpu[0] / 100})
				pusherData = append(pusherData, CountData{Time: now, Total: uint(rtsp.Instance.GetPusherSize())})
				playerCnt := 0
				for _, pusher := range rtsp.Instance.GetPushers() {
					playerCnt += pusher.GetPlayers().Len()
				}
				playerData = append(playerData, CountData{Time: now, Total: uint(playerCnt)})

				if len(memData) > timeSize {
					memData = memData[len(memData)-timeSize:]
				}
				if len(cpuData) > timeSize {
					cpuData = cpuData[len(cpuData)-timeSize:]
				}
				if len(pusherData) > timeSize {
					pusherData = pusherData[len(pusherData)-timeSize:]
				}
				if len(playerData) > timeSize {
					playerData = playerData[len(playerData)-timeSize:]
				}
			}
		}
	}()
}

/**
 * @api {get} /api/v1/getserverinfo 获取平台运行信息
 * @apiGroup sys
 * @apiName GetServerInfo
 * @apiSuccess (200) {String} Hardware 硬件信息
 * @apiSuccess (200) {String} RunningTime 运行时间
 * @apiSuccess (200) {String} StartUpTime 启动时间
 * @apiSuccess (200) {String} Server 软件信息
 */
func (h *APIHandler) GetServerInfo(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, gin.H{
		"Hardware":         strings.ToUpper(runtime.GOARCH),
		"InterfaceVersion": "V1",
		"RunningTime":      utils.UpTimeString(),
		"StartUpTime":      utils.DateTime(utils.StartTime),
		"Server":           fmt.Sprintf("%s/%s,%s (Platform/%s;)", "EasyDarwin", BuildDateTime, BuildVersion, strings.Title(runtime.GOOS)),
		"memData":          memData,
		"cpuData":          cpuData,
		"pusherData":       pusherData,
		"playerData":       playerData,
	})
}

/**
 * @api {get} /api/v1/restart 重启服务
 * @apiGroup sys
 * @apiName Restart
 * @apiUse simpleSuccess
 */
func (h *APIHandler) Restart(c *gin.Context) {
	log.Println("Restart...")
	c.JSON(http.StatusOK, "OK")
	go func() {
		select {
		case h.RestartChan <- true:
		default:
		}
	}()
}
