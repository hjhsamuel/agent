package skillx

import (
	"context"
	"fmt"
	"io"

	"github.com/hjhsamuel/agent/pkg/skillx"
	"google.golang.org/adk/v2/tool/skilltoolset/skill"
)

type Skill struct {
	names   []*skillx.Frontmatter
	nameMap map[string]struct{}
}

func (s *Skill) ListFrontmatters(ctx context.Context) ([]*skill.Frontmatter, error) {
	out := make([]*skill.Frontmatter, 0, len(s.names))
	for _, item := range s.names {
		out = append(out, &skill.Frontmatter{
			Name:        item.Name,
			Description: item.Description,
		})
	}

	return out, nil
}

func (s *Skill) ListResources(ctx context.Context, name, subpath string) ([]string, error) {
	if err := s.checkName(name); err != nil {
		return nil, err
	}
	return skillx.ListResources(name, subpath)
}

func (s *Skill) LoadFrontmatter(ctx context.Context, name string) (*skill.Frontmatter, error) {
	if err := s.checkName(name); err != nil {
		return nil, err
	}

	for _, item := range s.names {
		if name == item.Name {
			return &skill.Frontmatter{
				Name:        item.Name,
				Description: item.Description,
			}, nil
		}
	}
	return nil, skill.ErrSkillNotFound
}

func (s *Skill) LoadInstructions(ctx context.Context, name string) (string, error) {
	if err := s.checkName(name); err != nil {
		return "", err
	}
	_, content, err := skillx.ReadSkill(name, true)
	if err != nil {
		return "", err
	}

	return content, nil
}

func (s *Skill) LoadResource(ctx context.Context, name, resourcePath string) (io.ReadCloser, error) {
	if err := s.checkName(name); err != nil {
		return nil, err
	}
	return skillx.OpenResource(name, resourcePath)
}

func (s *Skill) checkName(name string) error {
	if s.nameMap != nil {
		if _, ok := s.nameMap[name]; !ok {
			return fmt.Errorf("%w: %q", skill.ErrSkillNotFound, name)
		}
	}
	return nil
}

func NewSkillSource(names ...string) skill.Source {
	s := &Skill{
		names:   make([]*skillx.Frontmatter, 0),
		nameMap: make(map[string]struct{}),
	}

	for _, name := range names {
		fm, _, err := skillx.ReadSkill(name, false)
		if err != nil {
			continue
		}
		s.names = append(s.names, fm)
		s.nameMap[fm.Name] = struct{}{}
	}
	return s
}
