package skillx

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// openSkillRoot constrains both the skill name and any symlinks to skills/.
func openSkillRoot(name string) (*os.Root, error) {
	if name == "." || !fs.ValidPath(name) || !filepath.IsLocal(name) || strings.ContainsAny(name, `\:`) {
		return nil, fmt.Errorf("invalid skill name: %q", name)
	}

	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid skill path: %q", name)
	}
	if len(parts) == 2 {
		if err := ValidateName(parts[0]); err != nil {
			return nil, err
		}
		version, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || version <= 0 || strconv.FormatInt(version, 10) != parts[1] {
			return nil, fmt.Errorf("invalid skill version: %q", name)
		}
	}
	root, err := os.OpenRoot("skills")
	if err != nil {
		return nil, err
	}
	defer root.Close()

	return root.OpenRoot(name)
}

// resourcePath uses slash-separated paths on all platforms and limits resources
// to the three public resource directories. os.Root enforces the filesystem
// boundary separately, including when a path contains symbolic links.
func resourcePath(name string) (string, error) {
	clean := path.Clean(name)
	if !fs.ValidPath(clean) || !filepath.IsLocal(filepath.FromSlash(clean)) || strings.ContainsAny(name, `\:`) {
		return "", fmt.Errorf("invalid resource path: %q", name)
	}
	switch strings.SplitN(clean, "/", 2)[0] {
	case "references", "assets", "scripts":
		return clean, nil
	default:
		return "", fmt.Errorf("invalid resource path: must be within 'references/', 'assets/', or 'scripts/'")
	}
}
