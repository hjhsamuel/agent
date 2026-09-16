package request

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Handler[T1 any, T2 any] func(ctx *Context, req T1) (T2, error)

type Context struct {
	*gin.Context
	Auth *User
}

type User struct {
	ID    int
	Name  string
	Token string
	jwt.RegisteredClaims
}

type Response struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

func GetHandle[T1 any, T2 any](h Handler[*T1, T2], auth bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				ctx.JSON(http.StatusInternalServerError, &Response{
					Code:    http.StatusInternalServerError,
					Message: fmt.Sprintf("%v", err),
				})
			}
		}()

		var (
			req = new(T1)
			err error
		)
		err = ctx.BindUri(req)
		if err == nil {
			switch ctx.Request.Method {
			case http.MethodGet:
				err = ctx.ShouldBindQuery(req)
			case http.MethodPost:
				_ = ctx.BindQuery(req)
				if ctx.ContentType() == "multipart/form-data" {
					err = ctx.ShouldBind(req)
				} else {
					err = ctx.ShouldBindJSON(req)
				}
			default:
				err = ctx.ShouldBindJSON(req)
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				ctx.JSON(http.StatusInternalServerError, &Response{
					Code:    http.StatusInternalServerError,
					Message: err.Error(),
				})
				return
			}
		}

		subCtx := &Context{Context: ctx}
		if v, ok := ctx.Get(AuthKey); ok && v != nil {
			if claim, fok := v.(*User); fok {
				subCtx.Auth = claim
			}
		}
		if auth && subCtx.Auth == nil {
			ctx.JSON(http.StatusUnauthorized, &Response{
				Code:    http.StatusUnauthorized,
				Message: "Unauthorized",
			})
			return
		}

		response, err := h(subCtx, req)
		if err != nil {
			ctx.JSON(http.StatusOK, &Response{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
		} else {
			ctx.JSON(http.StatusOK, &Response{
				Code: http.StatusOK,
				Data: response,
			})
		}
	}
}
