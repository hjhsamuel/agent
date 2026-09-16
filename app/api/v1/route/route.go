package route

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/app/api/v1/handler"
	"github.com/hjhsamuel/agent/internal/service"
)

func Route(srv *service.Service, g *gin.RouterGroup) {
	api := handler.NewApi(srv)
	group := g.Group("/v1")

	conversationRoutes(api, group)
}
