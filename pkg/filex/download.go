package filex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func Download(ctx context.Context, baseUrl, dst string) error {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseUrl, nil)
	if err != nil {
		return err
	}

	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", response.Status)
	}

	_ = os.MkdirAll(filepath.Dir(dst), os.ModePerm)

	// 创建临时文件
	tmpFile, err := os.CreateTemp(filepath.Dir(dst), ".download-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err = io.Copy(tmpFile, response.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	return os.Rename(tmpPath, dst)
}
