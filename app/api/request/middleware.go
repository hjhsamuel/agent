package request

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(ctx *gin.Context) {
	user, err := ParseToken(ctx)
	if err == nil {
		ctx.Set(AuthKey, user)
	}

	ctx.Next()
}

func ParseToken(ctx *gin.Context) (*User, error) {
	var info string

	content := ctx.Request.Header.Get("Authorization")
	parts := strings.Fields(content)
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		info = parts[1]
	}

	if info != "" {
		claim := &User{}
		token, err := jwt.ParseWithClaims(info, claim, func(token *jwt.Token) (any, error) {
			salt := GetTokenSalt()
			return []byte(salt), nil
		})
		if err == nil && token.Valid {
			claim.Token = info
			return claim, nil
		}
	}

	return nil, errors.New("unauthorized")
}
