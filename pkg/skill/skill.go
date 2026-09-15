package skill

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SkillDir      = "skills"
	SkillFileName = "SKILLS.md"
)

type Skill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func LoadSkills() ([]*Skill, error) {
	entries, err := os.ReadDir(SkillDir)
	if err != nil {
		return nil, fmt.Errorf("read skills directory: %w", err)
	}
	skills := make([]*Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		s, err := loadSkill(filepath.Join("skills", entry.Name(), SkillFileName))
		if err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	return skills, nil
}

// loadSkill reads only the YAML frontmatter from a SKILLS.md file.
func loadSkill(path string) (*Skill, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open skill %q: %w", path, err)
	}
	defer file.Close()

	// Accept a UTF-8 BOM and both LF and CRLF line endings.
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read skill frontmatter %q: %w", path, err)
		}
		return nil, fmt.Errorf("skill %q: missing opening YAML frontmatter delimiter", path)
	}
	if strings.TrimRight(strings.TrimPrefix(scanner.Text(), "\ufeff"), " \t") != "---" {
		return nil, fmt.Errorf("skill %q: missing opening YAML frontmatter delimiter", path)
	}

	var frontmatter bytes.Buffer
	closed := false
	for scanner.Scan() {
		if strings.TrimRight(scanner.Text(), " \t") == "---" {
			closed = true
			break
		}
		frontmatter.WriteString(scanner.Text())
		frontmatter.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read skill frontmatter %q: %w", path, err)
	}
	if !closed {
		return nil, fmt.Errorf("skill %q: missing closing YAML frontmatter delimiter", path)
	}

	var result Skill
	if err := yaml.Unmarshal(frontmatter.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse skill frontmatter %q: %w", path, err)
	}
	if strings.TrimSpace(result.Name) == "" || strings.TrimSpace(result.Description) == "" {
		return nil, fmt.Errorf("skill %q: name and description must not be empty", path)
	}
	return &result, nil
}
