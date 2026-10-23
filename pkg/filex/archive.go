package filex

import (
	"compress/gzip"
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
	gzipped := strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz")
	if gzipped {
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
		case "tar":
			format = &archives.Tar{}
		default:
			return errors.New("unknown format")
		}
	}

	if err := os.MkdirAll(dst, os.ModePerm); err != nil {
		return err
	}

	f, err := os.OpenFile(src, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f
	if gzipped {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	}

	return format.Extract(ctx, reader, func(ctx context.Context, info archives.FileInfo) error {
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
			return os.MkdirAll(fp, os.ModePerm)
		} else {
			if err := os.MkdirAll(filepath.Dir(fp), os.ModePerm); err != nil {
				return err
			}
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
