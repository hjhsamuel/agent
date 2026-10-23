package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/entities"
	"github.com/hjhsamuel/agent/internal/store/schema"
)

// GetSessionList 分页获取当前用户的会话列表。
// @Summary 获取会话列表
// @Description 按会话 ID 倒序返回当前用户的非临时会话，使用响应中的 offset 查询下一页。
// @Tags 会话
// @Produce json
// @Security CookieAuth
// @Param offset query string false "上一页返回的会话索引，首次查询不传"
// @Param limit query int true "最大返回数量，范围为 1 到 50" minimum(1) maximum(50)
// @Success 200 {object} common.Response{data=entities.GetSessionListRsp} "会话列表"
// @Failure 400 {object} common.Response "limit 不在允许范围内"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "查询会话失败"
// @Router /v1/session [get]
func (a *Api) GetSessionList(c *gin.Context) {
	var req entities.GetSessionListReq
	_ = c.ShouldBindQuery(&req)
	if req.Limit <= 0 || req.Limit > 50 {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "out of range: limit. expect at (0, 50]",
		})
		return
	}

	auth := common.GetAuth(c)

	objs, err := a.srv.GetSessionList(strconv.Itoa(auth.ID), req.Offset, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get session list: %v", err),
		})
		return
	}

	var (
		rsp    = make([]*entities.SessionBase, 0, len(objs))
		offset = req.Offset
	)
	for _, obj := range objs {
		offset = obj.ID.Hex()
		rsp = append(rsp, &entities.SessionBase{
			ID:        offset,
			Name:      obj.Name,
			State:     int(obj.State),
			CreatedAt: obj.CreatedAt.Unix(),
			UpdatedAt: obj.UpdatedAt.Unix(),
		})
		offset = obj.ID.Hex()
	}
	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: &entities.GetSessionListRsp{
			Offset: offset,
			Data:   rsp,
		},
	})
}

// DeleteSession 删除当前用户的会话。
// @Summary 删除会话
// @Tags 会话
// @Produce json
// @Security CookieAuth
// @Param id path string true "会话 ID"
// @Success 200 {object} common.Response "删除成功"
// @Failure 400 {object} common.Response "缺少会话 ID"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "删除会话失败"
// @Router /v1/session/{id} [delete]
func (a *Api) DeleteSession(c *gin.Context) {
	var req entities.DeleteSessionReq
	_ = c.ShouldBindUri(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	auth := common.GetAuth(c)

	err := a.srv.DeleteSession(strconv.Itoa(auth.ID), req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("delete session: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
}

// Stream 订阅会话的 SSE 事件流。
// @Summary 订阅会话事件流
// @Description 未传 id 时为当前用户创建临时会话，传入 id 时订阅已有会话。
// @Description 首个 init 事件的 data 为会话 ID；后续事件包含 id、event、data，event 类型为 content、reasoning、hitl 或 end。
// @Description hitl 事件的 data 为包含 id 和 content 的 JSON 字符串；seq 用于断线重连，传入最后收到的事件序号。
// @Description 建立事件流前的错误返回 JSON；事件流开始后的错误通过关闭连接结束，不再返回 JSON。
// @Tags 会话
// @Produce text/event-stream,json
// @Security CookieAuth
// @Param id query string false "会话 ID，不传则创建临时会话"
// @Param seq query integer false "最后收到的 SSE 事件序号，0 表示首次连接" default(0) minimum(0)
// @Success 200 {string} string "SSE 事件流"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "创建、查询会话或建立事件流失败"
// @Router /v1/session/stream [get]
func (a *Api) Stream(c *gin.Context) {
	var req entities.StreamReq
	_ = c.ShouldBindQuery(&req)

	auth := common.GetAuth(c)

	var sessionId string
	if req.ID == "" {
		id, err := a.srv.CreateSession(strconv.Itoa(auth.ID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, &common.Response{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("create session: %v", err),
			})
			return
		}
		sessionId = id
	} else {
		obj, err := a.srv.GetSession(strconv.Itoa(auth.ID), req.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, &common.Response{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("get session: %v", err),
			})
			return
		}
		if obj == nil {
			c.JSON(http.StatusBadRequest, &common.Response{
				Code:    http.StatusBadRequest,
				Message: "session not found",
			})
			return
		}
		sessionId = req.ID
	}

	err := a.srv.Stream(c, sessionId, req.Seq)
	if err != nil {
		if c.Writer.Written() {
			// Once SSE starts, a JSON response would corrupt the event stream.
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("stream: %v", err),
		})
	}
}

// Chat 向会话发送消息。
// @Summary 发送会话消息
// @Description 提交消息并启动模型任务；模型输出通过会话 SSE 事件流返回。
// @Tags 会话
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "会话 ID"
// @Param request body entities.ChatReq true "对话消息，content 必填"
// @Success 200 {object} common.Response "消息已提交"
// @Failure 400 {object} common.Response "缺少会话 ID 或消息内容"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "提交消息失败"
// @Router /v1/session/{id}/message [post]
func (a *Api) Chat(c *gin.Context) {
	var req entities.ChatReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)

	if req.ID == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id and content are required",
		})
		return
	}

	auth := common.GetAuth(c)

	err := a.srv.Run(strconv.Itoa(auth.ID), req.ID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("chat: %v", err),
		})
		return
	}

	rsp := &common.Response{
		Code: http.StatusOK,
	}
	c.JSON(http.StatusOK, rsp)
}

// InputRequired 提交任务所需的用户补充内容。
// @Summary 提交用户补充内容
// @Tags 会话
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "会话 ID"
// @Param request body entities.InputRequiredReq true "用户补充内容，call_id 和 content 必填"
// @Success 200 {object} common.Response "提交成功"
// @Failure 400 {object} common.Response "缺少会话 ID、任务 ID 或补充内容"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "提交补充内容失败"
// @Router /v1/session/{id}/input [post]
func (a *Api) InputRequired(c *gin.Context) {
	var req entities.InputRequiredReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)
	if req.ID == "" || req.CallID == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id, call_id and content are required",
		})
		return
	}

	auth := common.GetAuth(c)

	err := a.srv.SetInputRequiredInfo(strconv.Itoa(auth.ID), req.ID, req.CallID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("set input required: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
}

// GetChatMessages 分页获取会话的历史消息。
// @Summary 获取会话消息列表
// @Description 按消息 ID 倒序返回历史消息，使用响应中的 offset 查询下一页。
// @Tags 会话
// @Produce json
// @Security CookieAuth
// @Param id path string true "会话 ID"
// @Param offset query int false "上一页返回的消息索引，首次查询不传"
// @Param limit query int false "最大返回数量"
// @Success 200 {object} common.Response{data=entities.GetChatMessagesRsp} "历史消息列表"
// @Failure 400 {object} common.Response "缺少会话 ID 或 limit 小于等于 0"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "查询历史消息失败"
// @Router /v1/session/{id}/message [get]
func (a *Api) GetChatMessages(c *gin.Context) {
	var req entities.GetChatMessagesReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindQuery(&req)

	if req.ID == "" || req.Limit <= 0 {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id, limit is required",
		})
		return
	}

	objs, err := a.srv.GetSessionMessages(req.ID, req.Offset, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get chat messages: %v", err),
		})
		return
	}

	rsp := &entities.GetChatMessagesRsp{
		Offset: req.Offset,
		Data:   make([]*entities.MessageInfo, 0),
	}
	for _, item := range objs {
		rsp.Offset = item.Seq
		rsp.Data = append(rsp.Data, &entities.MessageInfo{
			Role:      string(item.Role),
			Content:   item.Content,
			CreatedAt: item.CreatedAt.Unix(),
		})
	}

	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

// WaitInputList 获取会话中等待用户输入的任务。
// @Summary 获取待补充输入列表
// @Description 返回任务 ID 和提示内容，可使用任务 ID 作为 call_id 提交用户补充内容。
// @Tags 会话
// @Produce json
// @Security CookieAuth
// @Param id path string true "会话 ID"
// @Success 200 {object} common.Response{data=[]entities.WaitInputInfo} "待补充输入列表"
// @Failure 400 {object} common.Response "缺少会话 ID"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "查询会话失败"
// @Router /v1/session/{id}/input [get]
func (a *Api) WaitInputList(c *gin.Context) {
	var req entities.WaitInputListReq
	_ = c.ShouldBindUri(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
	}

	auth := common.GetAuth(c)

	obj, err := a.srv.GetSession(strconv.Itoa(auth.ID), req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get session: %v", err),
		})
		return
	}

	rsp := make([]*entities.WaitInputInfo, 0)
	if obj != nil && obj.State == schema.SessionWait {
		for _, item := range obj.Wait {
			rsp = append(rsp, &entities.WaitInputInfo{
				ID:      item.ID,
				Content: item.Hint,
			})
		}
	}

	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}
