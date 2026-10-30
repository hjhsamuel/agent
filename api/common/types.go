package common

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var (
	TokenKey  string
	TokenSalt string
)

const AuthKey = "AUTH"

func SetTokenKey(key string) {
	TokenKey = key
}

func GetTokenKey() string {
	return TokenKey
}

func SetTokenSalt(salt string) {
	TokenSalt = salt
}

func GetTokenSalt() string {
	return TokenSalt
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

type Auth struct {
	ID    int
	Name  string
	Token string
	jwt.RegisteredClaims
}

func GetAuth(c *gin.Context) *Auth {
	v, exists := c.Get(AuthKey)
	if !exists {
		return &Auth{}
	}
	return v.(*Auth)
}
