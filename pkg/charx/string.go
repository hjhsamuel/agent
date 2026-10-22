package charx

import (
	"crypto/rand"
	"errors"
)

const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandomString(n int) (string, error) {
	if n < 0 {
		return "", errors.New("invalid length")
	}

	result := make([]byte, n)
	buf := make([]byte, n)

	pos := 0

	for pos < n {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}

		for _, b := range buf {
			if b >= 248 {
				continue
			}

			result[pos] = charset[int(b)%62]
			pos++

			if pos == n {
				break
			}
		}
	}

	return string(result), nil
}
