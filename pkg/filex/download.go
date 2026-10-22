package filex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

func Download(ctx context.Context, baseUrl, dst string) (string, error) {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseUrl, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", response.Status)
	}

	name := GetFileName(response)
	if name == "" {
		return "", errors.New("unknown filename")
	}

	_ = os.MkdirAll(dst, os.ModePerm)

	// 创建临时文件
	tmpFile, err := os.CreateTemp(dst, ".download-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err = io.Copy(tmpFile, response.Body); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	dstPath := filepath.Join(dst, name)
	err = os.Rename(tmpPath, dstPath)
	if err != nil {
		return "", err
	}

	return dstPath, nil
}

func GetFileName(response *http.Response) string {
	disposition := response.Header.Get("Content-Disposition")
	if disposition != "" {
		_, params, err := mime.ParseMediaType(disposition)
		if err == nil {
			if filename, ok := params["filename"]; ok && filename != "" {
				return path.Base(filename)
			}
		}
	}

	u := response.Request.URL
	if name := path.Base(u.Path); name != "." && name != "/" {
		if decode, err := url.PathUnescape(name); err == nil {
			return decode
		}
		return name
	}

	return ""
}
