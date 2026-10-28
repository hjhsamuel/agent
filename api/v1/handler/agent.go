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

// AddMainAgent 新增主 Agent。
// @Summary 新增主 Agent
// @Description 名称、描述和提示词为空时使用默认值，智能程度默认为中等。
// @Tags Agent
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body entities.AddMainAgentReq false "新增主 Agent参数"
// @Success 200 {object} common.Response{data=string} "创建成功，data 为 ID"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "新增主 Agent失败"
// @Router /v1/agent/main [post]
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

// AddAgentGroup 新增 Agent 分组。
// @Summary 新增 Agent 分组
// @Description 创建主 Agent 下的分组；name、description、prompt 必填。
// @Tags Agent
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body entities.AddAgentGroupReq true "新增 Agent 分组参数"
// @Success 200 {object} common.Response{data=string} "创建成功，data 为 ID"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "新增 Agent 分组失败"
// @Router /v1/agent/group [post]
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

// AddSubAgent 新增子 Agent。
// @Summary 新增子 Agent
// @Description 在主 Agent 或分组下创建子 Agent；name、description、prompt 必填。
// @Tags Agent
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "父级 Agent ID"
// @Param request body entities.AddSubAgentReq true "新增子 Agent参数"
// @Success 200 {object} common.Response{data=string} "创建成功，data 为 ID"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "新增子 Agent失败"
// @Router /v1/agent/{id}/sub [post]
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

// ModifyAgent 更新 Agent。
// @Summary 更新 Agent
// @Description 按需更新配置；level 为 0（低）、1（中）或 2（高）。
// @Tags Agent
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "Agent ID"
// @Param request body entities.ModifyAgentReq true "更新 Agent参数"
// @Success 200 {object} common.Response "请求成功"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "更新 Agent失败"
// @Router /v1/agent/{id} [put]
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

// DeleteAgent 删除 Agent。
// @Summary 删除 Agent
// @Tags Agent
// @Produce json
// @Security CookieAuth
// @Param id path string true "Agent ID"
// @Success 200 {object} common.Response "请求成功"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "删除 Agent失败"
// @Router /v1/agent/{id} [delete]
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

// GetMainAgentInfo 获取主 Agent 信息。
// @Summary 获取主 Agent 信息
// @Description 未找到主 Agent 时返回 500。
// @Tags Agent
// @Produce json
// @Security CookieAuth
// @Success 200 {object} common.Response{data=entities.AgentInfo} "请求成功"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "获取主 Agent 信息失败"
// @Router /v1/agent/main [get]
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

// GetSubAgents 获取子 Agent 列表。
// @Summary 获取子 Agent 列表
// @Description 按 Agent ID 升序分页，使用返回的 offset 查询下一页。当前实现尚未绑定查询参数，且类型校验会始终返回 400。
// @Tags Agent
// @Produce json
// @Security CookieAuth
// @Param id path string true "父级 Agent ID"
// @Param offset query string false "上一页返回的游标，首次查询不传"
// @Param limit query int false "最大返回数量，0 表示不限制" minimum(0) maximum(50)
// @Param type query int true "Agent 类型：1 为分组，2 为子 Agent" Enums(1,2)
// @Success 200 {object} common.Response{data=entities.GetSubAgentsRsp} "请求成功"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "获取子 Agent 列表失败"
// @Router /v1/agent/{id}/sub [get]
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
