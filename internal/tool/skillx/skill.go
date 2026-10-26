package skillx

import (
	"context"
	"fmt"
	"io"

	"github.com/hjhsamuel/agent/pkg/skillx"
	"google.golang.org/adk/v2/tool/skilltoolset/skill"
)

type Skill struct {
	names map[string]struct{}
}

func (s *Skill) ListFrontmatters(ctx context.Context) ([]*skill.Frontmatter, error) {
	if s.names != nil {
		out := make([]*skill.Frontmatter, 0, len(s.names))
		for name := range s.names {
			fm, err := s.LoadFrontmatter(ctx, name)
			if err != nil {
				return nil, err
			}
			out = append(out, fm)
		}
		return out, nil
	}

	rsp, err := skillx.ListSkills()
	if err != nil {
		return nil, err
	}

	out := make([]*skill.Frontmatter, 0, len(rsp))
	for _, item := range rsp {
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
	rsp, _, err := skillx.ReadSkill(name, false)
	if err != nil {
		return nil, err
	}

	return &skill.Frontmatter{
		Name:        rsp.Name,
		Description: rsp.Description,
	}, nil
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
	if s.names != nil {
		if _, ok := s.names[name]; !ok {
			return fmt.Errorf("%w: %q", skill.ErrSkillNotFound, name)
		}
	}
	return nil
}

// NewSkillSource limits access to names when provided; no names exposes all skills.
func NewSkillSource(names ...string) skill.Source {
	s := &Skill{}
	if len(names) > 0 {
		s.names = make(map[string]struct{}, len(names))
		for _, name := range names {
			s.names[name] = struct{}{}
		}
	}
	return s
}
