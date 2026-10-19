package skillx

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/hjhsamuel/agent/pkg/skillx"
	"google.golang.org/adk/v2/tool/skilltoolset/skill"
)

type Skill struct {
}

func (s *Skill) ListFrontmatters(ctx context.Context) ([]*skill.Frontmatter, error) {
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
	return skillx.ListResources(name, subpath)
}

func (s *Skill) LoadFrontmatter(ctx context.Context, name string) (*skill.Frontmatter, error) {
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
	_, content, err := skillx.ReadSkill(name, true)
	if err != nil {
		return "", err
	}

	return content, nil
}

func (s *Skill) LoadResource(ctx context.Context, name, resourcePath string) (io.ReadCloser, error) {
	p := filepath.Join("skills", name, resourcePath)
	return os.Open(p)
}

func NewSkillSource() skill.Source {
	return &Skill{}
}
