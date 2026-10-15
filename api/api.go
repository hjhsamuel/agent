package api

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/hjhsamuel/agent/api/v1"
	"github.com/hjhsamuel/agent/internal/service"
)

func Route(srv *service.Service, e *gin.Engine) {
	group := e.Group("/cagent")

	v1.Route(srv, group)
}
