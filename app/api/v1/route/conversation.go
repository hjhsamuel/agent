package route

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/app/api/v1/handler"
	"github.com/hjhsamuel/agent/app/api/v1/request"
)

func conversationRoutes(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/conversation")

	group.GET("/stream", api.SSEStream)
	group.POST("/:id/chat", request.GetHandle(api.Chat, true))
}
