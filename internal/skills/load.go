package skills

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

func Discover() []*Skill {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}

	skills := make([]*Skill, 0)
	for _, entry := range entries {
		skill, err := parse(filepath.Join(skillsDir, entry.Name(), skillFileName))
		if err == nil {
			skills = append(skills, skill)
		}
	}
	return skills
}

func LoadSkill(name string) (*Skill, error) {
	path := filepath.Join(skillsDir, name)
	skill, err := parse(filepath.Join(path, skillFileName))
	if err != nil {
		return nil, err
	}
	skill.Reference = listReference(path)
	return skill, nil
}

func parse(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	content = bytes.ReplaceAll(content, []byte("\r"), []byte("\n"))

	lines := strings.Split(string(content), "\n")

	start := slices.IndexFunc(lines, func(s string) bool {
		return strings.TrimSpace(s) != ""
	})
	if start == -1 || strings.TrimSpace(lines[start]) != "---" {
		return nil, errors.New("invalid YamlFormatter")
	}

	end := slices.IndexFunc(lines[start+1:], func(s string) bool {
		return strings.TrimSpace(s) == "---"
	})
	if end == -1 {
		return nil, errors.New("invalid YamlFormatter")
	}
	end = start + 1 + end

	yfContent := strings.Join(lines[start+1:end], "\n")
	var formatter YamlFormatter
	if err = yaml.Unmarshal([]byte(yfContent), &formatter); err != nil {
		return nil, err
	}

	body := strings.Join(lines[end+1:], "\n")
	return &Skill{
		YamlFormatter: formatter,
		Body:          body,
		Path:          path,
	}, nil
}

func listReference(path string) []string {
	entries, err := os.ReadDir(filepath.Join(path, skillReference))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, filepath.Join(skillReference, entry.Name()))
	}
	return names
}

func LoadReference(name, path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(skillsDir, name, path))
}
