package skillx

import (
	"fmt"
	"os"
)

func ListSkills() ([]*Frontmatter, error) {
	entries, err := os.ReadDir("skills")
	if err != nil {
		return nil, fmt.Errorf("scan skills: %w", err)
	}

	fms := make([]*Frontmatter, 0)
	for _, entry := range entries {
		frontmatter, _, err := ReadSkill(entry.Name(), false)
		if err != nil {
			continue
		}
		fms = append(fms, frontmatter)
	}

	return fms, nil
}
