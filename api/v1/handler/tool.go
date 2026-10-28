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

// AddTool 新增工具。
// @Summary 新增工具
// @Description name 必填；kind 为 0（本地工具）、1（A2A）、2（MCP）或 3（Skill）。本地工具和 Skill 异步初始化，创建成功不代表已就绪。
// @Tags 工具
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body entities.AddToolReq true "新增工具参数"
// @Success 200 {object} common.Response{data=string} "创建成功，data 为 ID"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "新增工具失败"
// @Router /v1/tool [post]
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

// GetToolList 查询工具列表。
// @Summary 查询工具列表
// @Description 通过 JSON 请求体传入筛选和分页条件；limit 范围为 0 到 50，0 表示不限制数量；响应 offset 用于下一页查询。
// @Tags 工具
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body entities.GetToolListReq false "查询工具列表参数"
// @Success 200 {object} common.Response{data=entities.GetToolListRsp} "请求成功"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "查询工具列表失败"
// @Router /v1/tool/find [post]
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

// GetToolInfo 获取工具详情。
// @Summary 获取工具详情
// @Description 返回工具基本信息及关联的 Agent ID 和名称。
// @Tags 工具
// @Produce json
// @Security CookieAuth
// @Param id path string true "工具 ID"
// @Success 200 {object} common.Response{data=entities.GetToolInfoRsp} "请求成功"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "获取工具详情失败"
// @Router /v1/tool/{id} [get]
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

// UpdateToolState 更新工具状态。
// @Summary 更新工具状态
// @Description state 为 0（准备中）、1（上线）或 2（下线）。
// @Tags 工具
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "工具 ID"
// @Param request body entities.UpdateToolStateReq true "更新工具状态参数"
// @Success 200 {object} common.Response "请求成功"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "更新工具状态失败"
// @Router /v1/tool/{id}/state [put]
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

// UpdateTool 更新工具。
// @Summary 更新工具
// @Description 按需更新工具名称、地址和关联 Agent。
// @Tags 工具
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "工具 ID"
// @Param request body entities.UpdateToolReq true "更新工具参数"
// @Success 200 {object} common.Response "请求成功"
// @Failure 400 {object} common.Response "必填参数缺失或参数无效"
// @Failure 401 {object} common.Response "认证 Cookie 缺失或无效"
// @Failure 500 {object} common.Response "更新工具失败"
// @Router /v1/tool/{id} [put]
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
