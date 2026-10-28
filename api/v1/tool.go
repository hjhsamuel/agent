package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common/middleware"
	"github.com/hjhsamuel/agent/api/v1/handler"
)

func toolRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/tool")
	group.Use(middleware.AuthMiddleware)

	group.POST("", api.AddTool)
	group.POST("/find", api.GetToolList)
	group.GET("/:id", api.GetToolInfo)
	group.PUT("/:id/state", api.UpdateToolState)
	group.PUT("/:id", api.UpdateTool)
}
