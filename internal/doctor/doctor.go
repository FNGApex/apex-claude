// Package doctor runs deterministic integrity checks on the plugin layout and
// project state. Exit 1 if any hard check fails — a command can branch on it.
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"apexclaude/internal/proj"
	"apexclaude/internal/signals"
)

// Run executes the checks, writing a report to w. Returns 0 if all pass, else 1.
func Run(w io.Writer) int {
	root := pluginRoot()
	fmt.Fprintf(w, "plugin root: %s\n\n", root)

	ok := true
	check := func(label string, pass bool) {
		mark := "✓"
		if !pass {
			mark = "✗"
			ok = false
		}
		fmt.Fprintf(w, "  %s %s\n", mark, label)
	}

	check("plugin.json is valid JSON", validJSON(filepath.Join(root, ".claude-plugin", "plugin.json")))
	check("hooks/hooks.json is valid JSON", validJSON(filepath.Join(root, "hooks", "hooks.json")))
	check("output-styles/ has a style", countGlob(root, "output-styles", "*.md") >= 1)
	check("agents/ has an agent", countGlob(root, "agents", "*.md") >= 1)
	check("commands/ has a command", countGlob(root, "commands", "*.md") >= 1)
	check("skills/ has a SKILL.md", countSkills(root) >= 1)

	// Project-state is reported as info, not a hard failure of the plugin itself.
	if code, reason := signals.Stale(proj.Root()); code == 0 {
		check("project signals fresh", true)
	} else {
		fmt.Fprintf(w, "  • signals: %s\n", reason)
	}

	fmt.Fprintln(w)
	if ok {
		fmt.Fprintln(w, "doctor: all checks passed")
		return 0
	}
	fmt.Fprintln(w, "doctor: failures above")
	return 1
}

// pluginRoot prefers $CLAUDE_PLUGIN_ROOT, else infers the repo root as the parent
// of the binary's bin/ directory (bin/apex -> repo root).
func pluginRoot() string {
	if r := os.Getenv("CLAUDE_PLUGIN_ROOT"); r != "" {
		return r
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(filepath.Dir(exe))
	}
	wd, _ := os.Getwd()
	return wd
}

func validJSON(p string) bool {
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return json.Valid(b)
}

func countGlob(root, dir, pattern string) int {
	m, _ := filepath.Glob(filepath.Join(root, dir, pattern))
	return len(m)
}

func countSkills(root string) int {
	m, _ := filepath.Glob(filepath.Join(root, "skills", "*", "SKILL.md"))
	return len(m)
}
