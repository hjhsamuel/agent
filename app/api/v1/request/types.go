package request

import "sync"

var (
	AuthKey   = "AUTH"
	tokenLock sync.RWMutex
	tokenSalt string
)

func SetTokenSalt(salt string) {
	tokenLock.Lock()
	defer tokenLock.Unlock()

	tokenSalt = salt
}

func GetTokenSalt() string {
	tokenLock.RLock()
	defer tokenLock.RUnlock()

	return tokenSalt
}
