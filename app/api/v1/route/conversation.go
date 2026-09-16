package route

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/app/api/v1/handler"
)

func conversationRoutes(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/conversation")

	group.GET("/:id/stream", api.SSEStream)
}
