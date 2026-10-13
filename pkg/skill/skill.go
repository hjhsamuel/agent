package skill

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	frontmatterSeparator    = []byte("---\n")
	frontmatterSeparatorWin = []byte("---\r\n")
)

func ReadSkill(name string, withContent bool) (*Frontmatter, string, error) {
	path := filepath.Join("skills", name, "SKILL.md")

	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	frontmatter, err := ParseFrontmatter(reader)
	if err != nil {
		return nil, "", err
	}

	if withContent {
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, "", err
		}
		return frontmatter, string(content), nil
	}

	return frontmatter, "", nil
}

func ParseFrontmatter(r *bufio.Reader) (*Frontmatter, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("reading first line: %w", err)
	}

	if !bytes.Equal(line, frontmatterSeparator) && !bytes.Equal(line, frontmatterSeparatorWin) {
		return nil, errors.New("invalid frontmatter separator")
	}

	yamlBytes := bytes.Buffer{}
	for {
		yamlLine, err := r.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return nil, errors.New("missing closing frontmatter separator")
		}
		if err != nil {
			return nil, fmt.Errorf("read a line of frontmatter: %w", err)
		}

		if bytes.Equal(yamlLine, frontmatterSeparator) || bytes.Equal(yamlLine, frontmatterSeparatorWin) {
			break
		}
		yamlBytes.Write(yamlLine)
	}

	var fmYaml Frontmatter
	decoder := yaml.NewDecoder(bytes.NewReader(yamlBytes.Bytes()))
	decoder.KnownFields(true)
	if err = decoder.Decode(&fmYaml); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	return &fmYaml, nil
}
