package skill

import (
	"os"
	"path/filepath"
)

func ReadResource(name, resource string) ([]byte, error) {
	path := filepath.Join("skills", name, resource)
	return os.ReadFile(path)
}
