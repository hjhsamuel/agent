package provider

import (
	"encoding/json"
	"errors"
)

func ExtractJson(content string) (string, error) {
	start := 0
	for start < len(content) {
		if content[start] == '{' || content[start] == '[' {
			break
		}
		start += 1
	}

	if start == len(content) {
		return "", errors.New("invalid json")
	}

	end := len(content) - 1
	for end >= start {
		if content[end] == '}' || content[end] == ']' {
			break
		}
		end -= 1
	}

	if end < start {
		return "", errors.New("invalid json")
	}

	raw := content[start : end+1]
	if !json.Valid([]byte(raw)) {
		return "", errors.New("invalid json")
	}

	return raw, nil
}
