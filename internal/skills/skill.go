package skills

const (
	skillsDir      = "skills"
	skillFileName  = "SKILL.md"
	skillReference = "references"
)

type Skill struct {
	YamlFormatter
	Body      string
	Reference []string
	Path      string
}

type YamlFormatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}
