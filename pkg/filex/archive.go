package filex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
)

func Extract(ctx context.Context, src, dst string) error {
	var format archives.Extractor
	if strings.HasSuffix(src, ".tar.gz") {
		format = &archives.Tar{}
	} else {
		index := strings.LastIndex(src, ".")
		if index == -1 {
			return errors.New("unknown format")
		}
		switch src[index+1:] {
		case "zip":
			format = &archives.Zip{}
		case "rar":
			format = &archives.Rar{}
		case "7z":
			format = &archives.SevenZip{}
		case "tar", "tgz":
			format = &archives.Tar{}
		default:
			return errors.New("unknown format")
		}
	}

	_ = os.MkdirAll(dst, os.ModePerm)

	f, err := os.OpenFile(src, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	return format.Extract(ctx, f, func(ctx context.Context, info archives.FileInfo) error {
		name := strings.ReplaceAll(info.NameInArchive, "\\", "/")

		if strings.HasPrefix(name, "/") ||
			strings.Contains(name, ":") {
			return fmt.Errorf("invalid path: %s", name)
		}

		name = path.Clean(name)
		if name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("invalid path: %s", name)
		}

		fp := filepath.Join(dst, name)
		if info.IsDir() {
			_ = os.MkdirAll(fp, os.ModePerm)
		} else {
			ff, err := info.Open()
			if err != nil {
				return err
			}
			defer ff.Close()

			w, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			if err != nil {
				return err
			}
			defer w.Close()

			_, err = io.Copy(w, ff)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
