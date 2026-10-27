package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/entities"
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
