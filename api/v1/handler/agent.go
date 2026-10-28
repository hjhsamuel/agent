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
