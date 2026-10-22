package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/v1/handler"
	"github.com/hjhsamuel/agent/internal/service"
)

func Route(srv *service.Service, g *gin.RouterGroup) {
	api := handler.NewApi(srv)
	group := g.Group("/v1")

	sessionRoute(api, group)
	modelRoute(api, group)
	secretRoute(api, group)
	toolRoute(api, group)
}
