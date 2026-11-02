package token

import "github.com/golang-jwt/jwt/v5"

func Generate(obj jwt.Claims, salt string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, obj)
	return token.SignedString([]byte(salt))
}
