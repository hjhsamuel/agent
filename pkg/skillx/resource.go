package skillx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func ReadResource(name, resource string) ([]byte, error) {
	fp := filepath.Join("skills", name, resource)
	return os.ReadFile(fp)
}

func ListResources(name, subPath string) ([]string, error) {
	root := filepath.Join("skills", name)
	skillFS := os.DirFS(root)

	cleanPath := path.Clean(subPath)
	isRoot := cleanPath == "." || cleanPath == ""

	var targets []string
	if !isRoot {
		switch strings.SplitN(cleanPath, "/", 2)[0] {
		case "references", "assets", "scripts":
		default:
			return nil, errors.New("invalid resource path: must be empty, root (.), or within 'references/', 'assets/', or 'scripts/'")
		}

		targets = []string{cleanPath}
	} else {
		targets = []string{"references", "assets", "scripts"}
	}

	var resources []string
	for _, target := range targets {
		err := fs.WalkDir(skillFS, target, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}

			if !d.IsDir() {
				resources = append(resources, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk target %q: %w", target, err)
		}
	}

	return resources, nil
}
