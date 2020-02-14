package routers

import (
	"fmt"
	"strings"

	"github.com/EasyDarwin/EasyDarwin/rtsp"
	"github.com/EasyDarwin/EasyDarwin/utils"
	"github.com/gin-gonic/gin"
)

/**
 * @apiDefine stats 统计
 */

/**
 * @apiDefine playerInfo
 * @apiSuccess (200) {String} rows.id
 * @apiSuccess (200) {String} rows.path
 * @apiSuccess (200) {String} rows.transType 传输模式
 * @apiSuccess (200) {Number} rows.inBytes 入口流量
 * @apiSuccess (200) {Number} rows.outBytes 出口流量
 * @apiSuccess (200) {String} rows.startAt 开始时间
 */

/**
 * @api {get} /api/v1/pushers 获取推流列表
 * @apiGroup stats
 * @apiName Pushers
 * @apiParam {Number} [start] 分页开始,从零开始
 * @apiParam {Number} [limit] 分页大小
 * @apiParam {String} [sort] 排序字段
 * @apiParam {String=ascending,descending} [order] 排序顺序
 * @apiParam {String} [q] 查询参数
 * @apiSuccess (200) {Number} total 总数
 * @apiSuccess (200) {Array} rows 推流列表
 * @apiSuccess (200) {String} rows.id
 * @apiSuccess (200) {String} rows.path
 * @apiSuccess (200) {String} rows.transType 传输模式
 * @apiSuccess (200) {Number} rows.inBytes 入口流量
 * @apiSuccess (200) {Number} rows.outBytes 出口流量
 * @apiSuccess (200) {String} rows.startAt 开始时间
 * @apiSuccess (200) {Number} rows.onlines 在线人数
 */
func (h *APIHandler) Pushers(c *gin.Context) {
	form := NewPageRequest()
	if err := c.Bind(form); err != nil {
		return
	}

	hostname := utils.GetRequestHostname(c.Request)

	stats := make([]interface{}, 0)
	for it := rtsp.Instance.GetPushers().Iterator(); !it.Done(); {
		_, _pusher := it.Next()
		pusher := _pusher.(rtsp.Pusher)

		port := pusher.Server().TCPPort
		rtsp := fmt.Sprintf("rtsp://%s:%d%s", hostname, port, pusher.Path())
		if port == 554 {
			rtsp = fmt.Sprintf("rtsp://%s%s", hostname, pusher.Path())
		}
		if form.Q != "" && !strings.Contains(strings.ToLower(rtsp), strings.ToLower(form.Q)) {
			continue
		}
		stats = append(stats, map[string]interface{}{
			"id":        pusher.ID(),
			"url":       rtsp,
			"path":      pusher.Path(),
			"source":    pusher.Source(),
			"transType": pusher.TransType(),
			"inBytes":   pusher.InBytes(),
			"outBytes":  pusher.OutBytes(),
			"startAt":   utils.DateTime(pusher.StartAt()),
			"onlines":   pusher.GetPlayers().Len(),
		})
	}
	pr := NewPageResponse(stats)
	if form.Sort != "" {
		pr.Sort(form.Sort, form.Order)
	}
	pr.Slice(form.Start, form.Limit)
	c.IndentedJSON(200, pr)
}

/**
 * @api {get} /api/v1/players 获取拉流列表
 * @apiGroup stats
 * @apiName Players
 * @apiParam {Number} [start] 分页开始,从零开始
 * @apiParam {Number} [limit] 分页大小
 * @apiParam {String} [sort] 排序字段
 * @apiParam {String=ascending,descending} [order] 排序顺序
 * @apiParam {String} [q] 查询参数
 * @apiSuccess (200) {Number} total 总数
 * @apiSuccess (200) {Array} rows 推流列表
 * @apiSuccess (200) {String} rows.id
 * @apiSuccess (200) {String} rows.path
 * @apiSuccess (200) {String} rows.transType 传输模式
 * @apiSuccess (200) {Number} rows.inBytes 入口流量
 * @apiSuccess (200) {Number} rows.outBytes 出口流量
 * @apiSuccess (200) {String} rows.startAt 开始时间
 */
func (h *APIHandler) Players(c *gin.Context) {
	form := NewPageRequest()
	if err := c.Bind(form); err != nil {
		return
	}
	players := make([]rtsp.Player, 0)
	for it := rtsp.Instance.GetPushers().Iterator(); !it.Done(); {
		_, _pusher := it.Next()
		pusher := _pusher.(rtsp.Pusher)

		_players := pusher.GetPlayers()
		for itPlayer := _players.Iterator(); !itPlayer.Done(); {
			_, _player := itPlayer.Next()
			player := _player.(rtsp.Player)
			players = append(players, player)
		}
	}
	_players := make([]interface{}, 0)
	for i := 0; i < len(players); i++ {
		player := players[i]
		path := player.Path()
		_players = append(_players, map[string]interface{}{
			"id":        player.ID(),
			"path":      path,
			"transType": player.TransType().String(),
			"inBytes":   player.InBytes(),
			"outBytes":  player.OutBytes(),
			"startAt":   utils.DateTime(player.StartAt()),
		})
	}
	pr := NewPageResponse(_players)
	if form.Sort != "" {
		pr.Sort(form.Sort, form.Order)
	}
	pr.Slice(form.Start, form.Limit)
	c.IndentedJSON(200, pr)
}
