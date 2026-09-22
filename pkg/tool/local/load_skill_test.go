package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillReadStaysInsideRootAndLimitsSize(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "skills")
	if err := os.MkdirAll(filepath.Join(root, "demo"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "demo", "SKILL.md"), []byte("instructions"), 0600); err != nil {
		t.Fatal(err)
	}
	l := &LoadSkill{root: root, skillFile: "SKILL.md"}
	result, err := l.Execute(context.Background(), "", `{"name":"demo"}`)
	if err != nil || result.Content != "instructions" {
		t.Fatalf("valid skill: %+v %v", result, err)
	}
	if _, err := l.Execute(context.Background(), "", `{"name":"demo","path":"../../outside"}`); err == nil {
		t.Fatal("path traversal allowed")
	}
	if err := os.WriteFile(filepath.Join(root, "demo", "large"), []byte(strings.Repeat("x", 128*1024+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Execute(context.Background(), "", `{"name":"demo","path":"large"}`); err == nil {
		t.Fatal("unbounded file read")
	}
}
