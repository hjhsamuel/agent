package local

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Cleanup removes unreferenced versions before any runners or downloads start.
// The tools directory must be exclusively owned by this service instance.
func Cleanup(directory string, versions map[string]int64) error {
	for name, version := range versions {
		if err := ValidateName(name); err != nil {
			return err
		}
		if version < 0 {
			return fmt.Errorf("invalid version for tool %q", name)
		}
	}
	root, err := os.OpenRoot(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	// Validate all retained directories before deleting anything.
	for name, version := range versions {
		if version == 0 {
			continue
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("tool %q is not a directory", name)
		}
		info, err = root.Lstat(filepath.Join(name, strconv.FormatInt(version, 10)))
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("tool %q version %d is not a directory", name, version)
		}
	}
	entries, err := readRootEntries(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if ValidateName(name) != nil {
			continue
		}
		version, exists := versions[name]
		if !exists {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				if err := root.RemoveAll(name); err != nil {
					return err
				}
			}
			continue
		}
		if !entry.IsDir() {
			continue
		}
		child, err := root.OpenRoot(name)
		if err != nil {
			return err
		}
		err = cleanupVersions(child, version)
		child.Close()
		if err != nil {
			return fmt.Errorf("clean tool %q: %w", name, err)
		}
	}
	return nil
}

func readRootEntries(root *os.Root) ([]os.DirEntry, error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func cleanupVersions(root *os.Root, current int64) error {
	entries, err := readRootEntries(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		version, parseErr := strconv.ParseInt(name, 10, 64)
		isVersion := parseErr == nil && version > 0 && strconv.FormatInt(version, 10) == name
		if (isVersion && version != current) || strings.HasPrefix(name, ".staging-") {
			if err := root.RemoveAll(name); err != nil {
				return err
			}
		}
	}
	return nil
}
