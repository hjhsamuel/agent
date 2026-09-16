package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/app/api/v1/request"
	"github.com/hjhsamuel/agent/app/api/v1/schema"
)

func (a *Api) SSEStream(c *gin.Context) {
	user, err := request.ParseToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, &request.Response{
			Code:    http.StatusUnauthorized,
			Message: err.Error(),
		})
		return
	}

	var req schema.SSEStreamReq
	if err = c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, &request.Response{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	if err = c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, &request.Response{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	err = a.srv.ListenSSE(c, strconv.Itoa(user.ID), req.ID, req.Seq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &request.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}
}
