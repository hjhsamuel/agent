package config

import "os"

func GetEnv(key, value string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return value
}
