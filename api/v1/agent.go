package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common/middleware"
	"github.com/hjhsamuel/agent/api/v1/handler"
)

func agentRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/agent")
	group.Use(middleware.AuthMiddleware)

	group.POST("/main", api.AddMainAgent)
	group.POST("/group", api.AddAgentGroup)
	group.POST("/:id/sub", api.AddSubAgent)
	group.PUT("/:id", api.ModifyAgent)
	group.DELETE("/:id", api.DeleteAgent)
}
