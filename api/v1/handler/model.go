package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/schema"
	"github.com/hjhsamuel/agent/internal/store/request"
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
	var req schema.GetModelInfoReq
	_ = c.ShouldBindUri(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	objs, err := a.srv.GetApiKeys(req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get api keys: %v", err),
		})
		return
	}

	rsp := make([]*schema.ApiKeyInfo, 0, len(objs))
	for _, item := range objs {
		rsp = append(rsp, &schema.ApiKeyInfo{
			ID:      item.ID,
			Name:    item.Name,
			Version: item.Version,
			Weight:  item.Weight,
			Enable:  item.Enable,
		})
	}

	c.JSON(http.StatusOK, &common.Response{
		Code: http.StatusOK,
		Data: rsp,
	})
}

func (a *Api) UpdateModel(c *gin.Context) {
	var req schema.UpdateModelReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id is required",
		})
		return
	}

	update := &request.UpdateModelReq{
		Provider:     req.Provider,
		Name:         req.Name,
		Url:          req.Url,
		ContextLimit: req.ContextLimit,
		Enable:       req.Enable,
	}
	if req.Reasoning != nil && req.Reasoning.Key != "" && req.Reasoning.Value != "" {
		update.Reasoning = &struct {
			Key   string
			Value string
		}{
			Key:   req.Reasoning.Key,
			Value: req.Reasoning.Value,
		}
	}

	err := a.srv.UpdateModel(req.ID, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("update model: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
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

	id, err := a.srv.AddApiKey(req.ID, req.Name, req.Key, req.Weight)
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

func (a *Api) DeleteApiKey(c *gin.Context) {
	var req schema.DeleteApiKeyReq
	_ = c.ShouldBindUri(&req)

	if req.ID == "" || req.Api == 0 {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id and api are required",
		})
		return
	}

	err := a.srv.DeleteApiKey(req.ID, req.Api)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("delete api key: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
}

func (a *Api) UpdateApiKey(c *gin.Context) {
	var req schema.UpdateApiKeyReq
	_ = c.ShouldBindUri(&req)
	_ = c.ShouldBindJSON(&req)

	if req.ID == "" || req.Api == 0 {
		c.JSON(http.StatusBadRequest, &common.Response{
			Code:    http.StatusBadRequest,
			Message: "invalid argument: id and api are required",
		})
		return
	}

	update := &request.UpdateApiKeyReq{
		Name:    req.Name,
		Upgrade: req.Upgrade,
		Weight:  req.Weight,
		Enable:  req.Enable,
	}

	err := a.srv.UpdateApiKey(req.ID, req.Api, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("update api key: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
		})
	}
}
