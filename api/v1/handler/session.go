package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/schema"
)

func (a *Api) GetSessionList(c *gin.Context) {
	var req schema.GetSessionListReq
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
		rsp    = make([]*schema.SessionBase, 0, len(objs))
		offset = req.Offset
	)
	for _, obj := range objs {
		offset = obj.ID.Hex()
		rsp = append(rsp, &schema.SessionBase{
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
		Data: &schema.GetSessionListRsp{
			Offset: offset,
			Data:   rsp,
		},
	})
}

func (a *Api) DeleteSession(c *gin.Context) {
	// TODO
}

func (a *Api) Stream(c *gin.Context) {
	var req schema.StreamReq
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
		_, err := a.srv.GetSession(strconv.Itoa(auth.ID), req.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, &common.Response{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("get session: %v", err),
			})
			return
		}
		sessionId = req.ID
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	err := a.srv.Stream(c, sessionId, req.Seq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("stream: %v", err),
		})
	}
}

func (a *Api) Chat(c *gin.Context) {
	// TODO
}

func (a *Api) InputRequired(c *gin.Context) {
	// TODO
}

func (a *Api) GetChatMessages(c *gin.Context) {
	// TODO
}

func (a *Api) WaitInputList(c *gin.Context) {
	// TODO
}
