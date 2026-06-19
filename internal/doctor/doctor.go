// Package doctor runs deterministic integrity checks on the Apex artifact layout
// and project state. Exit 1 if any hard check fails — a command can branch on it.
//
// Apex ships in two layouts and doctor checks the right contract for each:
//   - dev/plugin layout: the repo (or a marketplace plugin dir). Carries a
//     .claude-plugin/plugin.json manifest and hooks/hooks.json.
//   - loose install: ~/.claude, populated by scripts/install.sh. There is NO
//     plugin manifest and NO hooks/hooks.json — hooks are wired into
//     settings.json — so those checks are skipped and replaced by a
//     settings.json hook-wiring check.
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"apexclaude/internal/proj"
	"apexclaude/internal/signals"
)

// Run executes the checks, writing a report to w. Returns 0 if all pass, else 1.
func Run(w io.Writer) int {
	root := artifactRoot()
	loose := isLooseInstall(root)
	layout := "dev/plugin layout"
	if loose {
		layout = "loose install"
	}
	fmt.Fprintf(w, "artifact root: %s (%s)\n\n", root, layout)

	ok := true
	check := func(label string, pass bool) {
		mark := "✓"
		if !pass {
			mark = "✗"
			ok = false
		}
		fmt.Fprintf(w, "  %s %s\n", mark, label)
	}

	if loose {
		// Loose model owns a different contract: no manifest, hooks live in
		// settings.json. Validate the wiring the installer is responsible for.
		check("apex hooks wired in settings.json", apexHooksWired(root))
	} else {
		check("plugin.json is valid JSON", validJSON(filepath.Join(root, ".claude-plugin", "plugin.json")))
		check("hooks/hooks.json is valid JSON", validJSON(filepath.Join(root, "hooks", "hooks.json")))
	}
	check("output-styles/ has a style", countGlob(root, "output-styles", "*.md") >= 1)
	check("agents/ has an agent", countGlob(root, "agents", "*.md") >= 1)
	check("commands/ has a command", countGlob(root, "commands", "*.md") >= 1)
	check("skills/ has a SKILL.md", countSkills(root) >= 1)

	// PATH reachability is info, not a hard failure: hooks invoke the binary by
	// absolute path, so Apex works regardless. It only matters for running `apex`
	// by hand. Warn when the binary's own directory is absent from $PATH.
	if dir, on := binOnPath(); !on {
		fmt.Fprintf(w, "  • PATH: %s is not on $PATH — add it to run `apex` directly\n", dir)
	} else {
		check("binary dir on $PATH", true)
	}

	// Project-state is reported as info, not a hard failure of the artifacts.
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

// artifactRoot prefers $CLAUDE_PLUGIN_ROOT, else infers the root as the parent
// of the binary's bin/ directory (bin/apex -> root). For a loose install that
// resolves to ~/.claude; for the repo it resolves to the repo root.
func artifactRoot() string {
	if r := os.Getenv("CLAUDE_PLUGIN_ROOT"); r != "" {
		return r
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(filepath.Dir(exe))
	}
	wd, _ := os.Getwd()
	return wd
}

// isLooseInstall reports whether root looks like a loose ~/.claude install
// rather than the repo / a plugin dir. The discriminator is the absence of a
// .claude-plugin manifest dir: install.sh never copies it.
func isLooseInstall(root string) bool {
	if _, err := os.Stat(filepath.Join(root, ".claude-plugin")); err == nil {
		return false
	}
	return true
}

// isApexHookCmd reports whether a hook command invokes `apex hooks`. The binary
// is `apex` on Unix and `apex.exe` on Windows, so match both — keying on the
// bare `apex hooks` substring would false-negative on every Windows install.
func isApexHookCmd(cmd string) bool {
	return strings.Contains(cmd, "apex hooks") || strings.Contains(cmd, "apex.exe hooks")
}

// apexHooksWired reports whether settings.json under root wires at least one
// apex hook (matching the `apex hooks` command the installer writes).
func apexHooksWired(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, "settings.json"))
	if err != nil {
		return false
	}
	var cfg struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if json.Unmarshal(b, &cfg) != nil {
		return false
	}
	for _, groups := range cfg.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if isApexHookCmd(h.Command) {
					return true
				}
			}
		}
	}
	return false
}

// binOnPath reports the directory holding the running binary and whether that
// directory appears in $PATH. Returns ("", true) if the executable can't be
// resolved — we don't warn on what we can't measure.
func binOnPath() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", true
	}
	dir := filepath.Dir(exe)
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == dir {
			return dir, true
		}
	}
	return dir, false
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
