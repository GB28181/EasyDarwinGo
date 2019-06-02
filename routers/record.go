package routers

import (
	"github.com/EasyDarwin/EasyDarwin/models"
	"github.com/EasyDarwin/EasyDarwin/record"
	"github.com/EasyDarwin/EasyDarwin/rtsp"
	"github.com/gin-gonic/gin"
)

// StartRecordResponse of StartRecordRequest
type StartRecordResponse struct {
	Code int    `form:"code" json:"code"`
	Msg  string `form:"msg" json:"msg"`
}

// StartRecord instace
func (h *APIHandler) StartRecord(c *gin.Context) {
	req := &models.Record{}
	if err := c.Bind(req); err != nil {
		c.IndentedJSON(200, &StartRecordResponse{
			Code: 400,
			Msg:  "Bad request",
		})
		return
	}

	pusher := rtsp.Instance.GetPusher(req.PlayPath)
	if nil == pusher {
		c.IndentedJSON(200, &StartRecordResponse{
			Code: 404,
			Msg:  "Media source not found according to playpath",
		})
		return
	}

	task := &record.Task{
		ID:       req.ID,
		PlayPath: req.PlayPath,
	}

	err := record.AddTask(task)
	if nil != err {
		c.IndentedJSON(200, &StartRecordResponse{
			Code: 400,
			Msg:  err.Error(),
		})
		return
	}

	recorder, err := rtsp.NewRecorder(task, pusher)
	if nil != err {
		c.IndentedJSON(200, &StartRecordResponse{
			Code: 400,
			Msg:  err.Error(),
		})
		return
	}

	if err := pusher.AddPlayer(recorder); nil != err {
		c.IndentedJSON(200, &StartRecordResponse{
			Code: 400,
			Msg:  err.Error(),
		})
		return
	}

	c.IndentedJSON(200, &StartRecordResponse{
		Code: 0,
		Msg:  "OK",
	})
}

// QueryRecordResponse of QueryRecordRequest
type QueryRecordResponse struct {
	Code int            `form:"code" json:"code"`
	Msg  string         `form:"msg" json:"msg"`
	Data []*record.Task `form:"data" json:"data"`
}

// QueryRecord instance
func (h *APIHandler) QueryRecord(c *gin.Context) {
	tasks := record.GetAllTasks()

	c.IndentedJSON(200, &QueryRecordResponse{
		Code: 0,
		Msg:  "OK",
		Data: tasks,
	})
}
