package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common/middleware"
	"github.com/hjhsamuel/agent/api/v1/handler"
)

func sessionRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/session")
	group.Use(middleware.AuthMiddleware)

	group.GET("/:id/stream", api.Stream)
}
