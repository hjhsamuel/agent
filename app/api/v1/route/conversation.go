package route

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/app/api/request"
	"github.com/hjhsamuel/agent/app/api/v1/handler"
)

func conversationRoutes(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/conversation")

	group.GET("/stream", api.SSEStream)
	group.POST("/:id/chat", request.GetHandle(api.Chat, true))
	group.POST("/:id/input", request.GetHandle(api.TaskInput, true))
	group.POST("/:id/cancel", request.GetHandle(api.Cancel, true))
}
