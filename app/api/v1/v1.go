package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/app/api/v1/route"
	"github.com/hjhsamuel/agent/internal/service"
)

func Register(srv *service.Service, g *gin.RouterGroup) {
	route.Route(srv, g)
}
