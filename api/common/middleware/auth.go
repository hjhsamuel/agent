package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hjhsamuel/agent/api/common"
)

func AuthMiddleware(ctx *gin.Context) {
	content, err := ctx.Cookie(common.GetTokenKey())
	if err != nil || content == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, &common.Response{
			Code:    http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	claim := &common.Auth{}
	t, err := jwt.ParseWithClaims(content, claim, func(token *jwt.Token) (any, error) {
		salt := common.GetTokenSalt()
		return []byte(salt), nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	if err != nil || t == nil || !t.Valid {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, &common.Response{
			Code:    http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	claim.Token = content

	ctx.Set(common.AuthKey, claim)
	ctx.Next()
}
