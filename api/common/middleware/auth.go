package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hjhsamuel/agent/api/common"
)

type Auth struct {
	ID   int
	Name string
	jwt.RegisteredClaims
}

func AuthMiddleware(ctx *gin.Context) {
	content, err := ctx.Cookie(common.GetTokenKey())
	if err != nil || content == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, &common.Response{
			Code:    http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	claim := &Auth{}
	t, err := jwt.ParseWithClaims(content, claim, func(token *jwt.Token) (any, error) {
		salt := common.GetTokenSalt()
		return []byte(salt), nil
	})
	if err == nil && t.Valid {
		ctx.Set(common.AuthKey, claim)
	}

	ctx.Next()
}
