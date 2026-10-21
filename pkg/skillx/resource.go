package skillx

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

func ReadResource(name, resource string) ([]byte, error) {
	f, err := OpenResource(name, resource)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

// OpenResource opens a resource without allowing paths or symlinks to escape
// the selected skill directory. The caller must close the returned file.
func OpenResource(name, resource string) (*os.File, error) {
	clean, err := resourcePath(resource)
	if err != nil {
		return nil, err
	}
	root, err := openSkillRoot(name)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	return root.Open(filepath.FromSlash(clean))
}

func ListResources(name, subPath string) ([]string, error) {
	root, err := openSkillRoot(name)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	skillFS := root.FS()

	cleanPath := path.Clean(subPath)
	isRoot := cleanPath == "." || cleanPath == ""

	var targets []string
	if !isRoot {
		cleanPath, err = resourcePath(subPath)
		if err != nil {
			return nil, err
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
