package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/schema"
)

func (a *Api) AddModel(c *gin.Context) {
	var req schema.AddModelReq
	_ = c.ShouldBindJSON(&req)

	if req.Provider == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: provider is required",
		})
		return
	}
	if req.Name == "" || req.Url == "" || req.Type == "" || req.ContextLimit == 0 {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: name, url, type, context_limit are required",
		})
		return
	}

	var (
		reasoningKey string
		reasoningVal string
	)
	if req.Reasoning != nil {
		reasoningKey = req.Reasoning.Key
		reasoningVal = req.Reasoning.Value
	}

	modelId, err := a.srv.AddModel(req.Provider, req.Name, req.Url, req.Type, req.ContextLimit,
		reasoningKey, reasoningVal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("add model: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
			Data: modelId,
		})
	}
}

func (a *Api) GetModels(c *gin.Context) {
	groups, err := a.srv.GetModelGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get models: %v", err),
		})
		return
	}

	rsp := make([]*schema.GetModelsRsp, 0)
	for _, item := range groups {
		for _, elem := range item.Models {
			info := &schema.GetModelsRsp{
				ID:       elem.ID.Hex(),
				Provider: item.Provider,
				Name:     elem.Name,
				Type:     elem.Type,
				Enable:   elem.Enable,
			}
			rsp = append(rsp, info)
		}
	}

	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

func (a *Api) GetModelInfo(c *gin.Context) {
	// TODO
}

func (a *Api) UpdateModel(c *gin.Context) {
	// TODO
}

func (a *Api) AddApiKey(c *gin.Context) {
	var req schema.AddApiKeyReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}
	if req.Key == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: key is required",
		})
		return
	}

	id, err := a.srv.AddApiKey(req.ID, req.Key, req.Weight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("add api key: %v", err),
		})
		return
	}
	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: id,
	})
}
