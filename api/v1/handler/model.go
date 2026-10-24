package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/entities"
	"github.com/hjhsamuel/agent/internal/store/request"
)

// AddModel 新增模型。
// @Summary 新增模型
// @Description 创建模型配置，成功后返回模型 ID；新建模型默认禁用。
// @Tags 模型
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body entities.AddModelReq true "模型配置，provider、name、url、type、context_limit 必填"
// @Success 200 {object} common.Response{data=string} "创建成功，data 为模型 ID"
// @Failure 400 {object} common.Response "缺少必填参数"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "创建模型失败"
// @Router /v1/model [post]
func (a *Api) AddModel(c *gin.Context) {
	var req entities.AddModelReq
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
		reasoningKey, reasoningVal, req.Level)
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

// GetModels 获取模型列表。
// @Summary 获取模型列表
// @Tags 模型
// @Produce json
// @Security CookieAuth
// @Success 200 {object} common.Response{data=[]entities.GetModelsRsp} "模型列表"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "查询模型失败"
// @Router /v1/model [get]
func (a *Api) GetModels(c *gin.Context) {
	groups, err := a.srv.GetModelGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("get models: %v", err),
		})
		return
	}

	rsp := make([]*entities.GetModelsRsp, 0)
	for _, item := range groups {
		for _, elem := range item.Models {
			info := &entities.GetModelsRsp{
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

// GetModelInfo 获取模型的 API Key 列表。
// @Summary 获取模型的 API Key 列表
// @Description 返回指定模型的 API Key 元信息，不包含 API Key 明文。
// @Tags 模型
// @Produce json
// @Security CookieAuth
// @Param id path string true "模型 ID"
// @Success 200 {object} common.Response{data=[]entities.ApiKeyInfo} "API Key 列表"
// @Failure 400 {object} common.Response "缺少模型 ID"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "查询 API Key 失败"
// @Router /v1/model/{id} [get]
func (a *Api) GetModelInfo(c *gin.Context) {
	var req entities.GetModelInfoReq
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

	rsp := make([]*entities.ApiKeyInfo, 0, len(objs))
	for _, item := range objs {
		rsp = append(rsp, &entities.ApiKeyInfo{
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

// UpdateModel 更新模型配置。
// @Summary 更新模型配置
// @Description 按需更新模型字段；reasoning 的 key 和 value 均非空时才更新深度思考配置。
// @Tags 模型
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "模型 ID"
// @Param request body entities.UpdateModelReq true "待更新的模型配置"
// @Success 200 {object} common.Response "更新成功"
// @Failure 400 {object} common.Response "缺少模型 ID"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "更新模型失败"
// @Router /v1/model/{id} [put]
func (a *Api) UpdateModel(c *gin.Context) {
	var req entities.UpdateModelReq
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

// AddApiKey 为模型新增 API Key。
// @Summary 新增模型 API Key
// @Description 加密保存 API Key，成功后返回 API Key ID；新建 API Key 默认禁用。
// @Tags 模型
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "模型 ID"
// @Param request body entities.AddApiKeyReq true "API Key 配置，key 必填"
// @Success 200 {object} common.Response{data=int64} "创建成功，data 为 API Key ID"
// @Failure 400 {object} common.Response "缺少模型 ID 或 API Key"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "新增 API Key 失败"
// @Router /v1/model/{id}/key [post]
func (a *Api) AddApiKey(c *gin.Context) {
	var req entities.AddApiKeyReq
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

// DeleteApiKey 删除模型的 API Key。
// @Summary 删除模型 API Key
// @Tags 模型
// @Produce json
// @Security CookieAuth
// @Param id path string true "模型 ID"
// @Param api path int64 true "API Key ID，不能为 0"
// @Success 200 {object} common.Response "删除成功"
// @Failure 400 {object} common.Response "缺少模型 ID 或 API Key ID"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "删除 API Key 失败"
// @Router /v1/model/{id}/key/{api} [delete]
func (a *Api) DeleteApiKey(c *gin.Context) {
	var req entities.DeleteApiKeyReq
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

// UpdateApiKey 更新模型的 API Key 配置。
// @Summary 更新模型 API Key
// @Description 按需更新名称、权重和启用状态；upgrade 为 true 时使用最新密钥重新加密 API Key。
// @Tags 模型
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "模型 ID"
// @Param api path int64 true "API Key ID，不能为 0"
// @Param request body entities.UpdateApiKeyReq true "待更新的 API Key 配置"
// @Success 200 {object} common.Response "更新成功"
// @Failure 400 {object} common.Response "缺少模型 ID 或 API Key ID"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "更新 API Key 失败"
// @Router /v1/model/{id}/key/{api} [put]
func (a *Api) UpdateApiKey(c *gin.Context) {
	var req entities.UpdateApiKeyReq
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
