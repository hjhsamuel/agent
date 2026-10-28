package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/entities"
	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (a *Api) AddMainAgent(c *gin.Context) {
	var req entities.AddMainAgentReq
	_ = c.ShouldBindJSON(&req)

	id, err := a.srv.AddAgent(&schema.Agent{
		Name:        req.Name,
		Description: req.Description,
		Prompt:      req.Prompt,
		Type:        schema.AgentTypeMain,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("add agent: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
			Data: id,
		})
	}
}

func (a *Api) AddAgentGroup(c *gin.Context) {
	var req entities.AddAgentGroupReq
	_ = c.ShouldBindJSON(&req)

	if req.Name == "" || req.Description == "" || req.Prompt == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: name, description, prompt are required",
		})
		return
	}

	id, err := a.srv.AddAgent(&schema.Agent{
		Name:        req.Name,
		Description: req.Description,
		Prompt:      req.Prompt,
		Type:        schema.AgentTypeGroup,
		Level:       schema.AgentLevel(req.Level),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("add agent: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
			Data: id,
		})
	}
}

func (a *Api) AddSubAgent(c *gin.Context) {
	var req entities.AddSubAgentReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)

	if req.Name == "" || req.Description == "" || req.Prompt == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: name, description, prompt are required",
		})
		return
	}
	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	pid, err := bson.ObjectIDFromHex(req.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is invalid",
		})
		return
	}

	id, err := a.srv.AddAgent(&schema.Agent{
		Name:        req.Name,
		Description: req.Description,
		Prompt:      req.Prompt,
		Type:        schema.AgentTypeSub,
		Parent:      &pid,
		Level:       schema.AgentLevel(req.Level),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("add agent: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
			Data: id,
		})
	}
}

func (a *Api) ModifyAgent(c *gin.Context) {
	var req entities.ModifyAgentReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	err := a.srv.UpdateAgent(req.ID, &request.UpdateAgentReq{
		Name:        req.Name,
		Description: req.Description,
		Prompt:      req.Prompt,
		Level:       req.Level,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("modify agent: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
}

func (a *Api) DeleteAgent(c *gin.Context) {
	var req entities.DeleteAgentReq
	_ = c.ShouldBindUri(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	err := a.srv.DeleteAgent(req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("delete agent: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
}

func (a *Api) GetMainAgentInfo(c *gin.Context) {
	objs, err := a.srv.GetAgents(&request.GetAgentsReq{
		Type: int(schema.AgentTypeMain),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get main agent info: %v", err),
		})
		return
	}
	if len(objs) == 0 {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: "main agent not found",
		})
		return
	}

	agentObj := objs[0]
	rsp := &entities.AgentInfo{
		ID:          agentObj.ID.Hex(),
		Name:        agentObj.Name,
		Description: agentObj.Description,
		Prompt:      agentObj.Prompt,
		Type:        int(agentObj.Type),
		Level:       int(agentObj.Level),
		CreatedAt:   agentObj.CreatedAt.Unix(),
	}
	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

func (a *Api) GetSubAgents(c *gin.Context) {
	var req entities.GetSubAgentsReq
	_ = c.ShouldBindUri(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}
	if req.Limit < 0 || req.Limit > 50 {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: limit out of range, expect [0, 50]",
		})
		return
	}
	if req.Type != int(schema.AgentTypeGroup) || req.Type != int(schema.AgentTypeSub) {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: type is invalid",
		})
		return
	}

	objs, err := a.srv.GetAgents(&request.GetAgentsReq{
		Offset: req.Offset,
		Limit:  req.Limit,
		Type:   req.Type,
		Parent: req.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get sub agents: %v", err),
		})
		return
	}

	rsp := &entities.GetSubAgentsRsp{
		Offset: req.Offset,
		Data:   make([]*entities.AgentInfo, 0),
	}
	for _, item := range objs {
		id := item.ID.Hex()
		rsp.Offset = id
		rsp.Data = append(rsp.Data, &entities.AgentInfo{
			ID:          id,
			Name:        item.Name,
			Description: item.Description,
			Prompt:      item.Prompt,
			Type:        int(item.Type),
			Level:       int(item.Level),
			CreatedAt:   item.CreatedAt.Unix(),
		})
	}

	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}
