// Package layout resolves where an Agent's files live. A repo is on the apex layout (.apex/ state, .agents/skills/,
// AGENTS.md) or the legacy Claude-only layout (.claude/ state and skills, CLAUDE.md); each Agent is found by its skill.
package layout

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"apex/internal/exit"
)

type Layout struct {
	Name    string // apex | legacy
	State   string // shared state dir, relative to the main checkout
	Skills  string // skills dir, relative to the main checkout
	Framing string // the framing file every harness reads
}

var (
	Apex   = Layout{Name: "apex", State: ".apex", Skills: filepath.Join(".agents", "skills"), Framing: "AGENTS.md"}
	Legacy = Layout{Name: "legacy", State: ".claude", Skills: filepath.Join(".claude", "skills"), Framing: "CLAUDE.md"}
)

// Reserved names are directories of .apex/ itself, so on the apex layout an Agent's private dir would collide.
var Reserved = []string{"ledger", "workers", "worktrees"}

func (l Layout) Skill(root, agent string) string {
	return filepath.Join(root, l.Skills, agent, "SKILL.md")
}

func (l Layout) Ledger(root, agent string) string {
	return filepath.Join(root, l.State, "ledger", agent)
}

// Private is the Agent's own notes dir.
func (l Layout) Private(root, agent string) string {
	if l.Name == Legacy.Name {
		return filepath.Join(root, ".claude", "apex", agent)
	}
	return filepath.Join(root, ".apex", agent)
}

// Shared is a repo-wide state file such as LIVE.md, git.md or tiers.md.
func (l Layout) Shared(root, file string) string { return filepath.Join(root, l.State, file) }

func (l Layout) FramingFile(root string) string { return filepath.Join(root, l.Framing) }

// Workers is where worker worktrees and their manifests live, whatever the layout.
func Workers(root string) string { return filepath.Join(root, ".apex", "workers") }

// ForAgent picks the layout of agent (already validated as a path segment) by where its skill lives, apex first. A
// ledger left behind on the other layout is a split ledger: reading one half would silently lose the other.
func ForAgent(root, agent string) (Layout, error) {
	for _, pair := range [][2]Layout{{Apex, Legacy}, {Legacy, Apex}} {
		l, other := pair[0], pair[1]
		ok, err := exists(l.Skill(root, agent))
		if err != nil {
			return Layout{}, err
		}
		if !ok {
			continue
		}
		if l.Name == Apex.Name && slices.Contains(Reserved, agent) {
			return Layout{}, fmt.Errorf("%w: agent name %q is reserved on the apex layout", exit.Usage, agent)
		}
		split, err := exists(other.Ledger(root, agent))
		if err != nil {
			return Layout{}, err
		}
		if split {
			return Layout{}, fmt.Errorf("%w: split ledger: %s has its skill at %s but a ledger at %s",
				exit.Corrupt, agent, l.Skill(root, agent), other.Ledger(root, agent))
		}
		return l, nil
	}
	return Layout{}, fmt.Errorf("%w: no agent %q (missing %s and %s): %w",
		exit.NotFound, agent, Apex.Skill(root, agent), Legacy.Skill(root, agent), fs.ErrNotExist)
}

// ForRepo picks the layout for repo-wide paths: apex once anything of it exists, legacy if only legacy state does,
// apex for a fresh repo. Workers live under .apex/ on either layout, so .apex/workers alone does not count.
func ForRepo(root string) Layout {
	if ok, _ := exists(filepath.Join(root, Apex.Skills)); ok {
		return Apex
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".apex"))
	for _, e := range entries {
		if e.Name() != "workers" {
			return Apex
		}
	}
	for _, p := range []string{filepath.Join(".claude", "ledger"), filepath.Join(".claude", "apex")} {
		if ok, _ := exists(filepath.Join(root, p)); ok {
			return Legacy
		}
	}
	return Apex
}

type Root struct {
	Path   string
	Source string // repo | repo-claude | env | user | user-claude | user-codex
}

// Sources are the Root.Source values, in precedence order.
var Sources = []string{"repo", "repo-claude", "env", "user", "user-claude", "user-codex"}

// SkillRoots lists every place skills are discovered, highest precedence first.
func SkillRoots(root string) []Root {
	roots := []Root{
		{filepath.Join(root, Apex.Skills), "repo"},
		{filepath.Join(root, Legacy.Skills), "repo-claude"},
	}
	for _, p := range filepath.SplitList(os.Getenv("APEX_SKILLS_DIR")) {
		if p != "" {
			roots = append(roots, Root{p, "env"})
		}
	}
	home, _ := os.UserHomeDir()
	under := func(env string, rel ...string) string {
		if v := os.Getenv(env); env != "" && v != "" {
			return filepath.Join(v, "skills")
		}
		if home == "" {
			return ""
		}
		return filepath.Join(append([]string{home}, rel...)...)
	}
	for _, r := range []Root{
		{under("", ".agents", "skills"), "user"},
		{under("CLAUDE_CONFIG_DIR", ".claude", "skills"), "user-claude"},
		{under("CODEX_HOME", ".codex", "skills"), "user-codex"},
	} {
		if r.Path != "" {
			roots = append(roots, r)
		}
	}
	return roots
}

func exists(p string) (bool, error) {
	_, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
