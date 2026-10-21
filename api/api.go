package api

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/hjhsamuel/agent/api/v1"
	"github.com/hjhsamuel/agent/docs"
	"github.com/hjhsamuel/agent/internal/service"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Route(srv *service.Service, e *gin.Engine) {
	group := e.Group("/cagent")

	swaggerRoute(group)
	v1.Route(srv, group)
}

func swaggerRoute(e *gin.RouterGroup) {
	docs.SwaggerInfo.BasePath = "/cagent"
	e.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
