package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common/middleware"
	"github.com/hjhsamuel/agent/api/v1/handler"
)

func modelRoute(api *handler.Api, g *gin.RouterGroup) {
	group := g.Group("/model")
	group.Use(middleware.AuthMiddleware)

	group.POST("", api.AddModel)
	group.GET("", api.GetModels)
	group.GET("/:id", api.GetModelInfo)
	group.PUT("/:id", api.UpdateModel)

	group.POST("/:id/key", api.AddApiKey)
	group.DELETE("/:id/key/:api", api.DeleteApiKey)
	group.PUT("/:id/key", api.UpgradeApiKey)
}
