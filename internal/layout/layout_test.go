package layout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"apex/internal/exit"
)

func mk(t *testing.T, root string, rel ...string) {
	t.Helper()
	p := filepath.Join(append([]string{root}, rel...)...)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestForAgentPicksLayoutBySkill(t *testing.T) {
	root := t.TempDir()
	mk(t, root, ".agents", "skills", "new", "SKILL.md")
	mk(t, root, ".claude", "skills", "old", "SKILL.md")
	mk(t, root, ".claude", "skills", "new", "SKILL.md") // apex wins over a legacy copy
	if l, err := ForAgent(root, "new"); err != nil || l != Apex {
		t.Fatalf("new = %v, %v; want apex", l, err)
	}
	if l, err := ForAgent(root, "old"); err != nil || l != Legacy {
		t.Fatalf("old = %v, %v; want legacy", l, err)
	}
	_, err := ForAgent(root, "ghost")
	if !errors.Is(err, exit.NotFound) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ghost err = %v, want NotFound and ErrNotExist", err)
	}
	if got, want := Apex.Ledger(root, "new"), filepath.Join(root, ".apex", "ledger", "new"); got != want {
		t.Errorf("Apex.Ledger = %s, want %s", got, want)
	}
	if got, want := Legacy.Private(root, "old"), filepath.Join(root, ".claude", "apex", "old"); got != want {
		t.Errorf("Legacy.Private = %s, want %s", got, want)
	}
	if got, want := Apex.Private(root, "new"), filepath.Join(root, ".apex", "new"); got != want {
		t.Errorf("Apex.Private = %s, want %s", got, want)
	}
}

func TestForAgentSplitLedgerIsCorrupt(t *testing.T) {
	root := t.TempDir()
	mk(t, root, ".agents", "skills", "a", "SKILL.md")
	mk(t, root, ".claude", "ledger", "a", "001.md")
	mk(t, root, ".claude", "skills", "b", "SKILL.md")
	mk(t, root, ".apex", "ledger", "b", "001.md")
	for _, agent := range []string{"a", "b"} {
		if _, err := ForAgent(root, agent); !errors.Is(err, exit.Corrupt) {
			t.Errorf("%s: err = %v, want Corrupt", agent, err)
		}
	}
}

func TestForAgentReservedOnApex(t *testing.T) {
	root := t.TempDir()
	mk(t, root, ".agents", "skills", "workers", "SKILL.md")
	mk(t, root, ".claude", "skills", "ledger", "SKILL.md")
	if _, err := ForAgent(root, "workers"); !errors.Is(err, exit.Usage) {
		t.Errorf("apex workers: err = %v, want Usage", err)
	}
	if l, err := ForAgent(root, "ledger"); err != nil || l != Legacy {
		t.Errorf("legacy ledger: %v, %v; want legacy", l, err)
	}
}

func TestForRepo(t *testing.T) {
	root := t.TempDir()
	if ForRepo(root) != Apex {
		t.Fatal("fresh repo must be apex")
	}
	mk(t, root, ".claude", "ledger", "main", "001.md")
	if ForRepo(root) != Legacy {
		t.Fatal("legacy state only must be legacy")
	}
	mk(t, root, ".apex", "workers", "001.json")
	if ForRepo(root) != Legacy {
		t.Fatal("workers alone must not flip a legacy repo to apex")
	}
	mk(t, root, ".apex", "LIVE.md")
	if ForRepo(root) != Apex {
		t.Fatal(".apex present must be apex")
	}
}

func TestSkillRootsOrder(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APEX_SKILLS_DIR", "one"+string(os.PathListSeparator)+string(os.PathListSeparator)+"two")
	t.Setenv("CLAUDE_CONFIG_DIR", "cc")
	t.Setenv("CODEX_HOME", "cx")
	var got []string
	for _, r := range SkillRoots(root) {
		got = append(got, r.Source+"="+r.Path)
	}
	home, _ := os.UserHomeDir()
	want := []string{
		"repo=" + filepath.Join(root, ".agents", "skills"),
		"repo-claude=" + filepath.Join(root, ".claude", "skills"),
		"env=one", "env=two",
		"user=" + filepath.Join(home, ".agents", "skills"),
		"user-claude=" + filepath.Join("cc", "skills"),
		"user-codex=" + filepath.Join("cx", "skills"),
	}
	if len(got) != len(want) {
		t.Fatalf("roots = %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("root %d = %s, want %s", i, got[i], want[i])
		}
	}
}
