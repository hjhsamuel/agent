package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common/middleware"
	"github.com/hjhsamuel/agent/api/v1/handler"
)

func sessionRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/session")
	group.Use(middleware.AuthMiddleware)

	group.GET("", api.GetSessionList)
	group.DELETE("/:id", api.DeleteSession)

	group.GET("/stream", api.Stream)
	group.GET("/:id", api.GetSessionInfo)

	group.POST("/:id/message", api.Chat)
	group.GET("/:id/message", api.GetChatMessages)
	group.POST("/:id/input", api.InputRequired)
	group.GET("/:id/input", api.WaitInputList)
}
