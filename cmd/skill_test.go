package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSkillTargets(t *testing.T) {
	ts, err := skillTargets("global", "all")
	if err != nil || len(ts) != 3 {
		t.Fatalf("global/all: %d targets, err %v", len(ts), err)
	}
	want := map[string]string{
		"claude":  filepath.Join(".claude", "skills"),
		"copilot": filepath.Join(".copilot", "skills"),
		"agents":  filepath.Join(".agents", "skills"),
	}
	for _, tg := range ts {
		if !strings.HasSuffix(tg.dir, filepath.Join(want[tg.agent], "")) {
			t.Errorf("%s global dir = %q", tg.agent, tg.dir)
		}
	}

	// Local Copilot lives under .github/skills.
	ts, err = skillTargets("local", "copilot")
	if err != nil || len(ts) != 1 {
		t.Fatalf("local/copilot: %d targets, err %v", len(ts), err)
	}
	if !strings.HasSuffix(ts[0].dir, filepath.Join(".github", "skills")) {
		t.Errorf("local copilot dir = %q", ts[0].dir)
	}

	if _, err := skillTargets("global", "bogus"); err == nil {
		t.Error("expected error for invalid agent")
	}
	if _, err := skillTargets("bogus", "all"); err == nil {
		t.Error("expected error for invalid scope")
	}
}

func TestWriteSkill(t *testing.T) {
	oldFS, oldRoot := SkillFS, SkillEmbedRoot
	defer func() { SkillFS, SkillEmbedRoot = oldFS, oldRoot }()

	SkillEmbedRoot = "skills/aks-access"
	SkillFS = fstest.MapFS{
		"skills/aks-access/SKILL.md":           {Data: []byte("# skill body")},
		"skills/aks-access/reference/extra.md": {Data: []byte("nested")},
	}

	dest := filepath.Join(t.TempDir(), "aks-access")
	if err := writeSkill(dest); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "SKILL.md")); err != nil || string(b) != "# skill body" {
		t.Errorf("SKILL.md = %q err %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "reference", "extra.md")); err != nil || string(b) != "nested" {
		t.Errorf("nested file = %q err %v", b, err)
	}
}
