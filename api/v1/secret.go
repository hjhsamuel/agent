package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common/middleware"
	"github.com/hjhsamuel/agent/api/v1/handler"
)

func secretRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/secret")
	group.Use(middleware.AuthMiddleware)

	group.POST("", api.GenerateSecret)
}
