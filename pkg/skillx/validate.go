package skillx

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// ValidateName checks the skill name and its use as a local directory name.
func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 64 || !filepath.IsLocal(name) ||
		strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return fmt.Errorf("invalid skill name: %q", name)
	}
	for _, ch := range name {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return fmt.Errorf("invalid skill name: %q", name)
		}
	}
	return nil
}

// ValidateSkill validates an unpacked skill before installing it under name.
func ValidateSkill(directory, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read skill directory: %w", err)
	}
	found := false
	for _, entry := range entries {
		if entry.Name() == "SKILL.md" {
			if !entry.Type().IsRegular() {
				return errors.New("SKILL.md must be a regular file")
			}
			found = true
			break
		}
	}
	if !found {
		return errors.New("skill directory must contain SKILL.md")
	}

	f, err := os.Open(filepath.Join(directory, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("open SKILL.md: %w", err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	fm, err := ParseFrontmatter(reader)
	if err != nil {
		return err
	}

	if fm.Name != name {
		return fmt.Errorf("skill name %q must match installation directory %q", fm.Name, name)
	}
	if strings.TrimSpace(fm.Description) == "" || utf8.RuneCountInString(fm.Description) > 1024 {
		return errors.New("skill description must contain 1-1024 characters and not be blank")
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read skill instructions: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return errors.New("skill instructions must not be empty")
	}
	return nil
}

func IsSkillExist(name string) bool {
	_, err := os.Stat(filepath.Join("skills", name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		return true
	}
	return true
}
