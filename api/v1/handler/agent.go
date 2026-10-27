package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/entities"
	"github.com/hjhsamuel/agent/internal/store/schema"
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
	// TODO
}

func (a *Api) AddSubAgent(c *gin.Context) {
	// TODO
}

func (a *Api) ModifyAgent(c *gin.Context) {
	// TODO
}

func (a *Api) DeleteAgent(c *gin.Context) {
	// TODO
}
