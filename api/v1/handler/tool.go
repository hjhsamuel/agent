package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/entities"
	"github.com/hjhsamuel/agent/internal/store/request"
)

func (a *Api) AddTool(c *gin.Context) {
	var req entities.AddToolReq
	_ = c.ShouldBindJSON(&req)

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: name is required",
		})
		return
	}

	if _, err := url.Parse(req.Url); err != nil {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: url is invalid",
		})
		return
	}

	auth := common.GetAuth(c)

	id, err := a.srv.AddTool(strconv.Itoa(auth.ID), req.Name, req.Kind, req.Url, req.Agent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("add tool: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
			Data: id,
		})
	}
}

func (a *Api) GetToolList(c *gin.Context) {
	var req entities.GetToolListReq
	_ = c.ShouldBindJSON(&req)

	if req.Limit < 0 || req.Limit > 50 {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: limit is out of range, expect [0, 50]",
		})
		return
	}

	objs, err := a.srv.GetTools(&request.GetToolsReq{
		Agent:  req.Agent,
		Type:   req.Type,
		State:  req.State,
		Offset: req.Offset,
		Limit:  req.Limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get tool list: %v", err),
		})
		return
	}

	rsp := &entities.GetToolListRsp{
		Offset: req.Offset,
		Data:   make([]*entities.ToolOfList, 0),
	}
	for _, item := range objs {
		agentId := item.ID.Hex()

		rsp.Offset = agentId
		rsp.Data = append(rsp.Data, &entities.ToolOfList{
			ToolBase: entities.ToolBase{
				ID:        agentId,
				Name:      item.Name,
				Type:      int(item.Type),
				State:     int(item.State),
				Creator:   item.Creator,
				CreatedAt: item.CreatedAt.Unix(),
			},
			Link: item.Agent != nil,
		})
	}
	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

func (a *Api) GetToolInfo(c *gin.Context) {
	var req entities.GetToolInfoReq
	_ = c.ShouldBindUri(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	toolObj, agentObj, err := a.srv.GetToolInfo(req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get tool info: %v", err),
		})
		return
	}
	rsp := &entities.GetToolInfoRsp{
		ToolBase: entities.ToolBase{
			ID:        toolObj.ID.Hex(),
			Name:      toolObj.Name,
			Type:      int(toolObj.Type),
			State:     int(toolObj.State),
			Creator:   toolObj.Creator,
			CreatedAt: toolObj.CreatedAt.Unix(),
		},
	}
	if agentObj != nil {
		rsp.AgentName = agentObj.Name
		rsp.AgentID = agentObj.ID.Hex()
	}

	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

func (a *Api) UpdateToolState(c *gin.Context) {
	var req entities.UpdateToolStateReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	err := a.srv.UpdateToolState(req.ID, req.State)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("update tool state: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
}

func (a *Api) UpdateTool(c *gin.Context) {
	var req entities.UpdateToolReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	err := a.srv.UpdateToolInfo(req.ID, &request.UpdateToolReq{
		Name:  req.Name,
		Url:   req.Url,
		Agent: req.Agent,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("update tool: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
}
