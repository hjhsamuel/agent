package common

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
	Message string `json:"message"`
	Data    any    `json:"data"`
}
