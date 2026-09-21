package api

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/hjhsamuel/agent/app/api/v1"
	"github.com/hjhsamuel/agent/internal/service"
)

func Register(srv *service.Service, r *gin.Engine) {
	group := r.Group("/agent")

	v1.Register(srv, group)
}
