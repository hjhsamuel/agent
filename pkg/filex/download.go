package filex

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
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

	if err := os.MkdirAll(dst, os.ModePerm); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return "", err
	}
	defer root.Close()

	// 创建临时文件
	tmpName := ".download-" + rand.Text()
	tmpFile, err := root.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer root.Remove(tmpName)

	if _, err = io.Copy(tmpFile, response.Body); err != nil {
		tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	dstPath := filepath.Join(dst, name)
	err = root.Rename(tmpName, name)
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
				return safeFileName(filename)
			}
		}
	}

	if response.Request == nil || response.Request.URL == nil {
		return ""
	}
	// URL.Path is already unescaped by net/url. A second unescape could turn
	// a literal filename containing %2f or %5c back into a directory separator.
	return safeFileName(path.Base(response.Request.URL.Path))
}

func safeFileName(name string) string {
	if name == "" || name == "." || name == ".." || !filepath.IsLocal(name) ||
		strings.ContainsAny(name, `/\:<>"|?*`) || strings.TrimSpace(name) != name || strings.HasSuffix(name, ".") {
		return ""
	}
	for _, ch := range name {
		if ch < 32 || ch == 127 {
			return ""
		}
	}
	return name
}
