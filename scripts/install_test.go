package scripts_test

// Runs scripts/install.* and uninstall.* against a fixture repo in t.TempDir(): stub skills for
// every name in workflows.conf, a fake apex binary (--bin), a temp HOME, and a fake claude on PATH
// that only logs its calls. Windows runs the PowerShell scripts (pwsh, and powershell 5.1 if
// present); elsewhere the sh scripts. APEX_TEST_SH=1 also runs sh on Windows, best effort: Git sh
// reports /tmp-style paths and finds the real claude, so a few assertions fail there.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

type runner struct {
	name string
	exe  string
	ps   bool
}

func runners(t *testing.T) []runner {
	t.Helper()
	var rs []runner
	if runtime.GOOS == "windows" {
		for _, n := range []string{"pwsh", "powershell"} {
			if p, err := exec.LookPath(n); err == nil {
				rs = append(rs, runner{n, p, true})
			}
		}
	}
	if p, err := exec.LookPath("sh"); err == nil && (runtime.GOOS != "windows" || os.Getenv("APEX_TEST_SH") != "") {
		rs = append(rs, runner{"sh", p, false})
	}
	if len(rs) == 0 {
		t.Skip("no pwsh, powershell or sh on PATH")
	}
	return rs
}

func scriptsDir(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// readConf parses workflows.conf: "<workflow>: skill skill ...".
func readConf(t *testing.T, path string) map[string][]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("workflows.conf line %q is not '<workflow>: skills'", line)
		}
		m[name] = strings.Fields(rest)
	}
	return m
}

func union(lists ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range lists {
		for _, s := range l {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	sort.Strings(out)
	return out
}

func sorted(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}

func slash(s string) string { return strings.ReplaceAll(s, "\\", "/") }

type fx struct {
	t        *testing.T
	r        runner
	repo     string
	home     string
	claude   string // default Claude config dir under home
	fakeBin  string
	logPath  string
	extraEnv []string
	conf     map[string][]string
}

func newFx(t *testing.T, r runner) *fx {
	t.Helper()
	root := t.TempDir()
	f := &fx{t: t, r: r, repo: filepath.Join(root, "repo"), home: filepath.Join(root, "home")}
	f.claude = filepath.Join(f.home, ".claude")
	src := scriptsDir(t)
	f.conf = readConf(t, filepath.Join(src, "workflows.conf"))

	must(t, os.MkdirAll(filepath.Join(f.repo, "scripts"), 0o755))
	for _, pat := range []string{"*.sh", "*.ps1", "workflows.conf"} {
		files, _ := filepath.Glob(filepath.Join(src, pat))
		for _, p := range files {
			b, err := os.ReadFile(p)
			must(t, err)
			must(t, os.WriteFile(filepath.Join(f.repo, "scripts", filepath.Base(p)), b, 0o755))
		}
	}
	for _, s := range union(f.conf["orchestration"], f.conf["multi-agent"]) {
		write(t, filepath.Join(f.repo, "skills", s, "SKILL.md"), "stub "+s+"\n")
	}
	write(t, filepath.Join(f.repo, "output-styles", "apex.md"), "---\nname: Apex\n---\nstub\n")
	must(t, os.MkdirAll(filepath.Join(f.repo, "mods"), 0o755))
	must(t, os.MkdirAll(f.home, 0o755))

	f.fakeBin = filepath.Join(root, "fake-apex")
	write(t, f.fakeBin, "#!/bin/sh\necho apex\n")

	f.logPath = filepath.Join(root, "claude.log")
	write(t, f.logPath, "")
	cb := filepath.Join(root, "fakebin")
	if runtime.GOOS == "windows" {
		write(t, filepath.Join(cb, "claude.cmd"), "@echo off\r\necho %CLAUDE_CONFIG_DIR%^|%*>>\"%APEX_CLAUDE_LOG%\"\r\n")
	} else {
		write(t, filepath.Join(cb, "claude"), "#!/bin/sh\necho \"$CLAUDE_CONFIG_DIR|$*\" >> \"$APEX_CLAUDE_LOG\"\n")
		must(t, os.Chmod(filepath.Join(cb, "claude"), 0o755))
	}
	f.extraEnv = []string{"PATH=" + cb + string(os.PathListSeparator) + os.Getenv("PATH")}
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	must(t, os.WriteFile(path, []byte(content), 0o644))
}

func (f *fx) env() []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(k) {
		case "HOME", "USERPROFILE", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "XDG_CONFIG_HOME", "APEX_CLAUDE_LOG":
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "HOME="+f.home, "USERPROFILE="+f.home, "APEX_CLAUDE_LOG="+f.logPath)
	return append(env, f.extraEnv...)
}

// psCommand builds the -Command text that runs a script with sh-style flags turned into
// PowerShell ones (--with-output-style=no -> -WithOutputStyle:$false). -File would hand 5.1 the
// string "$false", which a switch rejects, so the flags go through -Command.
func psCommand(script string, args []string) string {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	words := []string{"&", quote(script)}
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			words = append(words, quote(a))
			continue
		}
		name, val, hasVal := strings.Cut(a[2:], "=")
		parts := strings.Split(name, "-")
		for i, p := range parts {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
		flag := "-" + strings.Join(parts, "")
		if hasVal && val == "no" {
			flag += ":$false"
		}
		words = append(words, flag)
	}
	// A script's `exit N` leaves $? false and the code in $LASTEXITCODE; a thrown error leaves no code.
	return strings.Join(words, " ") + "; if (-not $?) { if ($LASTEXITCODE) { exit $LASTEXITCODE } else { exit 1 } }; exit 0"
}

func (f *fx) script(name string, stdin string, args ...string) (int, string) {
	f.t.Helper()
	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if f.r.ps {
		a := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", psCommand(filepath.Join(f.repo, "scripts", name+".ps1"), args)}
		cmd = exec.CommandContext(ctx, f.r.exe, a...)
	} else {
		cmd = exec.CommandContext(ctx, f.r.exe, append([]string{filepath.Join(f.repo, "scripts", name+".sh")}, args...)...)
	}
	cmd.Dir = f.repo
	cmd.Env = f.env()
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		f.t.Fatalf("running %s: %v", name, err)
	}
	return code, string(out)
}

// install runs install with the fake binary appended.
func (f *fx) install(args ...string) (int, string) {
	return f.script("install", "", append(args, "--bin", f.fakeBin)...)
}
func (f *fx) uninstall(args ...string) (int, string) { return f.script("uninstall", "", args...) }

func (f *fx) expect(code int, out string, want int) {
	f.t.Helper()
	if code != want {
		f.t.Fatalf("exit %d, want %d; output:\n%s", code, want, out)
	}
}

func (f *fx) agentsDir() string { return filepath.Join(f.home, ".agents", "skills") }
func (f *fx) manifest() string  { return filepath.Join(f.home, ".config", "apex", "manifest") }
func (f *fx) binPath(dir string) string {
	n := "apex"
	if runtime.GOOS == "windows" {
		n = "apex.exe"
	}
	return filepath.Join(dir, n)
}
func (f *fx) defaultBin() string { return f.binPath(filepath.Join(f.home, ".local", "bin")) }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// marked lists the skill directories under dir that carry the ownership mark.
func marked(dir string) []string {
	var out []string
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if exists(filepath.Join(dir, e.Name(), ".apex-installed")) {
			out = append(out, e.Name())
		}
	}
	return sorted(out)
}

func (f *fx) wantMarked(dir string, want []string) {
	f.t.Helper()
	got, w := marked(dir), sorted(want)
	if strings.Join(got, " ") != strings.Join(w, " ") {
		f.t.Fatalf("marked skills in %s:\n got  %v\n want %v", dir, got, w)
	}
}

func (f *fx) manifestLines() []string {
	f.t.Helper()
	b, err := os.ReadFile(f.manifest())
	if err != nil {
		f.t.Fatalf("manifest: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "\r") {
		f.t.Fatalf("manifest has CR: %q", s)
	}
	if strings.HasPrefix(s, "\xef\xbb\xbf") || !strings.HasSuffix(s, "\n") {
		f.t.Fatalf("manifest has a BOM or no final newline: %q", s)
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func (f *fx) wantManifest(lines ...string) {
	f.t.Helper()
	got := f.manifestLines()
	for _, w := range lines {
		found := false
		for _, g := range got {
			if slash(g) == slash(w) {
				found = true
			}
		}
		if !found {
			f.t.Fatalf("manifest lacks %q:\n%s", w, strings.Join(got, "\n"))
		}
	}
}

func (f *fx) claudeLog() string {
	b, _ := os.ReadFile(f.logPath)
	return string(b)
}

func (f *fx) plugins(json string) {
	write(f.t, filepath.Join(f.claude, "plugins", "installed_plugins.json"), json)
}

// each runs fn as a parallel subtest per runner with a fresh fixture.
func each(t *testing.T, name string, fn func(t *testing.T, f *fx)) {
	for _, r := range runners(t) {
		t.Run(name+"/"+r.name, func(t *testing.T) {
			t.Parallel()
			fn(t, newFx(t, r))
		})
	}
}

func TestDefaultInstall(t *testing.T) {
	each(t, "default", func(t *testing.T, f *fx) {
		code, out := f.install("--yes")
		f.expect(code, out, 0)
		f.wantMarked(f.agentsDir(), f.conf["orchestration"])
		if len(marked(f.agentsDir())) != 8 {
			t.Fatalf("want 8 orchestration skills")
		}
		if !exists(f.defaultBin()) {
			t.Fatalf("binary not installed at %s", f.defaultBin())
		}
		f.wantManifest("version\t1", "workflow\torchestration", "bin\t"+f.defaultBin(), "config\t"+f.claude, "skills\tagents\t"+f.agentsDir())
		if exists(f.claude) {
			t.Fatalf("%s was created by an agents install", f.claude)
		}
		if f.claudeLog() != "" {
			t.Fatalf("claude was called: %q", f.claudeLog())
		}
		if strings.Contains(out, "keeping workflow") {
			t.Fatalf("first install should not claim a previous install:\n%s", out)
		}
	})
}

func TestMultiAgentOnClaude(t *testing.T) {
	each(t, "multi-agent-claude", func(t *testing.T, f *fx) {
		code, out := f.install("--workflow", "multi-agent", "--harness", "claude", "--yes")
		f.expect(code, out, 0)
		f.wantMarked(filepath.Join(f.claude, "skills"), f.conf["multi-agent"])
		if len(marked(filepath.Join(f.claude, "skills"))) != 7 {
			t.Fatalf("want 7 multi-agent skills")
		}
		if exists(filepath.Join(f.claude, "output-styles")) || exists(f.agentsDir()) {
			t.Fatalf("unrequested extras or agents dir written")
		}
		if f.claudeLog() != "" {
			t.Fatalf("claude was called: %q", f.claudeLog())
		}
	})
}

func TestCodexAndClaudeEnv(t *testing.T) {
	each(t, "env-dirs", func(t *testing.T, f *fx) {
		codex := filepath.Join(t.TempDir(), "codex-home")
		cc := filepath.Join(t.TempDir(), "claude-cfg")
		f.extraEnv = append(f.extraEnv, "CODEX_HOME="+codex, "CLAUDE_CONFIG_DIR="+cc)
		code, out := f.install("--harness", "codex", "--yes")
		f.expect(code, out, 0)
		f.wantMarked(filepath.Join(codex, "skills"), f.conf["orchestration"])
		code, out = f.install("--harness", "claude", "--yes")
		f.expect(code, out, 0)
		f.wantMarked(filepath.Join(cc, "skills"), f.conf["orchestration"])
		f.wantMarked(filepath.Join(codex, "skills"), nil) // switching harness swept codex
		if exists(f.claude) {
			t.Fatalf("default %s used despite CLAUDE_CONFIG_DIR", f.claude)
		}
		f.wantManifest("config\t" + cc)
	})
}

func TestCustomHarness(t *testing.T) {
	each(t, "custom", func(t *testing.T, f *fx) {
		dir := filepath.Join(t.TempDir(), "my skills dir")
		code, out := f.install("--skills-dir", dir, "--yes")
		f.expect(code, out, 0)
		f.wantMarked(dir, f.conf["orchestration"])
		f.wantManifest("skills\tcustom\t" + dir)

		g := newFx(t, f.r)
		code, out = g.install("--harness", "custom", "--yes")
		g.expect(code, out, 64)
		if exists(g.manifest()) || exists(g.defaultBin()) {
			t.Fatalf("usage error wrote files")
		}
	})
}

func TestBothOnTwoHarnesses(t *testing.T) {
	each(t, "both", func(t *testing.T, f *fx) {
		code, out := f.install("--harness", "agents,claude", "--workflow", "both", "--yes")
		f.expect(code, out, 0)
		want := union(f.conf["orchestration"], f.conf["multi-agent"])
		if len(want) != 9 {
			t.Fatalf("fixture union has %d skills, want 9", len(want))
		}
		f.wantMarked(f.agentsDir(), want)
		f.wantMarked(filepath.Join(f.claude, "skills"), want)
		// The same directory named twice is installed once.
		code, out = f.install("--harness", "claude,claude", "--workflow", "both", "--yes")
		f.expect(code, out, 0)
		f.wantMarked(filepath.Join(f.claude, "skills"), want)
	})
}

func TestOwnershipGuard(t *testing.T) {
	each(t, "ownership", func(t *testing.T, f *fx) {
		mine := filepath.Join(f.agentsDir(), "git-discipline", "notes.txt")
		write(t, mine, "mine")
		code, out := f.install("--yes")
		f.expect(code, out, 1)
		if b, _ := os.ReadFile(mine); string(b) != "mine" {
			t.Fatalf("user file touched: %q", b)
		}
		if exists(f.defaultBin()) || exists(f.manifest()) || exists(filepath.Join(f.agentsDir(), "sign-off")) {
			t.Fatalf("a failed preflight wrote files")
		}
	})
}

func TestUnrelatedSkillSurvives(t *testing.T) {
	each(t, "unrelated", func(t *testing.T, f *fx) {
		keep := filepath.Join(f.agentsDir(), "my-own-skill", "SKILL.md")
		write(t, keep, "mine")
		for _, args := range [][]string{{"--yes"}, {"--workflow", "multi-agent", "--yes"}} {
			code, out := f.install(args...)
			f.expect(code, out, 0)
			if !exists(keep) {
				t.Fatalf("unmarked skill removed by install %v", args)
			}
		}
		code, out := f.uninstall()
		f.expect(code, out, 0)
		if !exists(keep) {
			t.Fatalf("unmarked skill removed by uninstall")
		}
		f.wantMarked(f.agentsDir(), nil)
	})
}

func TestWorkflowSwitch(t *testing.T) {
	each(t, "workflow-switch", func(t *testing.T, f *fx) {
		code, out := f.install("--workflow", "orchestration", "--yes")
		f.expect(code, out, 0)
		code, out = f.install("--workflow", "multi-agent", "--yes")
		f.expect(code, out, 0)
		f.wantMarked(f.agentsDir(), f.conf["multi-agent"])
		for _, gone := range []string{"orchestrator-protocol", "worker-protocol"} {
			if exists(filepath.Join(f.agentsDir(), gone)) {
				t.Fatalf("%s left behind", gone)
			}
		}
		if !exists(filepath.Join(f.agentsDir(), "agent-protocol", "SKILL.md")) {
			t.Fatalf("agent-protocol not installed")
		}
	})
}

func TestHarnessSwitch(t *testing.T) {
	each(t, "harness-switch", func(t *testing.T, f *fx) {
		code, out := f.install("--harness", "claude", "--yes")
		f.expect(code, out, 0)
		f.wantMarked(filepath.Join(f.claude, "skills"), f.conf["orchestration"])
		code, out = f.install("--harness", "agents", "--yes")
		f.expect(code, out, 0)
		f.wantMarked(filepath.Join(f.claude, "skills"), nil)
		f.wantMarked(f.agentsDir(), f.conf["orchestration"])
		f.wantManifest("skills\tagents\t" + f.agentsDir())
	})
}

func TestPrefixMove(t *testing.T) {
	each(t, "prefix-move", func(t *testing.T, f *fx) {
		p1, p2 := filepath.Join(f.home, "p one"), filepath.Join(f.home, "p two")
		code, out := f.install("--prefix", p1, "--yes")
		f.expect(code, out, 0)
		code, out = f.install("--prefix", p2, "--yes")
		f.expect(code, out, 0)
		if exists(f.binPath(p1)) || !exists(f.binPath(p2)) {
			t.Fatalf("binary did not move from %s to %s", p1, p2)
		}
	})
}

func TestRerunKeepsPreviousSettings(t *testing.T) {
	each(t, "rerun", func(t *testing.T, f *fx) {
		code, out := f.install("--workflow", "multi-agent", "--harness", "claude", "--yes")
		f.expect(code, out, 0)
		code, out = f.install("--yes")
		f.expect(code, out, 0)
		if !strings.Contains(out, "keeping workflow multi-agent, harness claude from previous install") {
			t.Fatalf("no note about inherited settings:\n%s", out)
		}
		f.wantMarked(filepath.Join(f.claude, "skills"), f.conf["multi-agent"])
		f.wantManifest("workflow\tmulti-agent")
	})
	each(t, "legacy-pre-manifest", func(t *testing.T, f *fx) {
		for _, s := range []string{"agent-protocol", "git-discipline", "no-longer-shipped"} {
			write(t, filepath.Join(f.claude, "skills", s, "SKILL.md"), "old")
			write(t, filepath.Join(f.claude, "skills", s, ".apex-installed"), "")
		}
		code, out := f.install("--yes")
		f.expect(code, out, 0)
		if !strings.Contains(out, "keeping workflow multi-agent, harness claude") {
			t.Fatalf("legacy install not inherited:\n%s", out)
		}
		f.wantMarked(filepath.Join(f.claude, "skills"), f.conf["multi-agent"])
		f.wantMarked(f.agentsDir(), nil)
		f.wantManifest("workflow\tmulti-agent", "skills\tclaude\t"+filepath.Join(f.claude, "skills"))
	})
}

func TestClaudeExtras(t *testing.T) {
	each(t, "extras", func(t *testing.T, f *fx) {
		code, out := f.install("--harness", "claude", "--with-output-style", "--with-claude-mods", "--yes")
		f.expect(code, out, 0)
		style := filepath.Join(f.claude, "output-styles", "apex.md")
		if b, _ := os.ReadFile(style); !strings.Contains(string(b), "name: Apex") {
			t.Fatalf("style not installed: %q", b)
		}
		log := slash(f.claudeLog())
		for _, want := range []string{"plugin marketplace add", "plugin install token-spend@apex", "plugin install usage-footer@apex"} {
			if !strings.Contains(log, want) {
				t.Fatalf("claude log lacks %q:\n%s", want, log)
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
			if !strings.HasPrefix(line, slash(f.claude)+"|") {
				t.Fatalf("claude ran without CLAUDE_CONFIG_DIR=%s: %q", f.claude, line)
			}
		}
		// The fake claude installs nothing, so say what a real one would have recorded.
		f.plugins(`{"plugins":{"token-spend@apex":[{}],"usage-footer@apex":[{}]}}`)
		code, out = f.install("--harness", "claude", "--with-output-style=no", "--with-claude-mods=no", "--yes")
		f.expect(code, out, 0)
		if exists(style) {
			t.Fatalf("style not removed by =no")
		}
		log = f.claudeLog()
		for _, want := range []string{"plugin uninstall token-spend@apex", "plugin uninstall usage-footer@apex", "plugin marketplace remove apex"} {
			if !strings.Contains(log, want) {
				t.Fatalf("claude log lacks %q:\n%s", want, log)
			}
		}
	})
	each(t, "extras-foreign", func(t *testing.T, f *fx) {
		style := filepath.Join(f.claude, "output-styles", "apex.md")
		write(t, style, "---\nname: Mine\n---\n")
		code, out := f.install("--harness", "claude", "--with-output-style", "--yes")
		f.expect(code, out, 1)
		code, out = f.install("--harness", "claude", "--with-output-style=no", "--yes")
		f.expect(code, out, 0)
		if b, _ := os.ReadFile(style); !strings.Contains(string(b), "name: Mine") {
			t.Fatalf("foreign style touched: %q", b)
		}
		f.plugins(`{"plugins":{"token-spend@someone-else":[{}]}}`)
		code, out = f.install("--harness", "claude", "--with-claude-mods", "--yes")
		f.expect(code, out, 1)
		if f.claudeLog() != "" {
			t.Fatalf("claude was called: %q", f.claudeLog())
		}
	})
}

func TestPreflightChecks(t *testing.T) {
	each(t, "ponytail", func(t *testing.T, f *fx) {
		code, out := f.install("--check-ponytail", "--yes")
		f.expect(code, out, 1)
		f.plugins(`{"plugins":{"ponytail@ponytail":[{}]}}`)
		code, out = f.install("--check-ponytail", "--check", "--yes")
		f.expect(code, out, 0)
		if exists(f.manifest()) || exists(f.defaultBin()) || exists(f.agentsDir()) {
			t.Fatalf("--check wrote files")
		}
		code, out = f.install("--check-ponytail", "--yes")
		f.expect(code, out, 0)
		if !exists(f.defaultBin()) {
			t.Fatalf("install did not run")
		}
	})
	each(t, "legacy-binary", func(t *testing.T, f *fx) {
		for _, n := range []string{"apex", "apex.exe"} {
			legacy := filepath.Join(f.claude, "bin", n)
			write(t, legacy, "old")
			code, out := f.install("--yes")
			f.expect(code, out, 1)
			if !strings.Contains(out, "legacy") {
				t.Fatalf("no legacy warning:\n%s", out)
			}
			must(t, os.Remove(legacy))
		}
		if exists(f.manifest()) {
			t.Fatalf("failed preflight wrote a manifest")
		}
	})
}

func TestInteractive(t *testing.T) {
	each(t, "interactive", func(t *testing.T, f *fx) {
		// Multi-Agent Collaboration, claude, no output style, no mods, no ponytail, proceed.
		code, out := f.script("install", "2\n2\nn\nn\nn\ny\n", "--interactive", "--bin", f.fakeBin)
		f.expect(code, out, 0)
		for _, want := range []string{"Orchestration-based Workflow", "Multi-Agent Collaboration", "Will install:", "Proceed? [Y/n]"} {
			if !strings.Contains(out, want) {
				t.Fatalf("prompt output lacks %q:\n%s", want, out)
			}
		}
		f.wantMarked(filepath.Join(f.claude, "skills"), f.conf["multi-agent"])
		f.wantMarked(f.agentsDir(), nil)
		f.wantManifest("workflow\tmulti-agent")
		if exists(filepath.Join(f.claude, "output-styles")) || f.claudeLog() != "" {
			t.Fatalf("declined extras were installed")
		}
	})
	each(t, "interactive-decline", func(t *testing.T, f *fx) {
		code, out := f.script("install", "1\n1\nn\n", "--custom", "--bin", f.fakeBin)
		f.expect(code, out, 1)
		if exists(f.manifest()) || exists(f.defaultBin()) {
			t.Fatalf("declined install wrote files")
		}
	})
}

func TestUninstall(t *testing.T) {
	each(t, "uninstall", func(t *testing.T, f *fx) {
		keep := filepath.Join(f.claude, "skills", "my-own-skill", "SKILL.md")
		write(t, keep, "mine")
		code, out := f.install("--workflow", "both", "--harness", "agents,claude", "--with-output-style", "--yes")
		f.expect(code, out, 0)
		f.plugins(`{"plugins":{"token-spend@apex":[{}]}}`)
		code, out = f.uninstall()
		f.expect(code, out, 0)
		f.wantMarked(f.agentsDir(), nil)
		f.wantMarked(filepath.Join(f.claude, "skills"), nil)
		if exists(f.defaultBin()) || exists(f.manifest()) || exists(filepath.Dir(f.manifest())) {
			t.Fatalf("binary or manifest left behind")
		}
		if exists(filepath.Join(f.claude, "output-styles", "apex.md")) {
			t.Fatalf("style left behind")
		}
		if !exists(keep) {
			t.Fatalf("unmarked skill removed")
		}
		if !strings.Contains(f.claudeLog(), "plugin uninstall token-spend@apex") {
			t.Fatalf("mod not uninstalled:\n%s", f.claudeLog())
		}
		code, out = f.uninstall()
		f.expect(code, out, 0)
	})
	each(t, "uninstall-custom-dir", func(t *testing.T, f *fx) {
		dir := filepath.Join(t.TempDir(), "my skills")
		code, out := f.install("--skills-dir", dir, "--yes")
		f.expect(code, out, 0)
		code, out = f.uninstall()
		f.expect(code, out, 0)
		f.wantMarked(dir, nil)
	})
}

func TestInvalidChoices(t *testing.T) {
	each(t, "invalid", func(t *testing.T, f *fx) {
		for _, args := range [][]string{{"--workflow", "bogus"}, {"--harness", "bogus"}, {"--harness", "agents,"}} {
			code, out := f.install(append(args, "--yes")...)
			f.expect(code, out, 64)
		}
		if exists(f.manifest()) || exists(f.defaultBin()) {
			t.Fatalf("usage errors wrote files")
		}
	})
}

func TestScriptsShellSyntax(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not on PATH")
	}
	for _, n := range []string{"apex-install-common.sh", "install.sh", "uninstall.sh"} {
		if out, err := exec.Command(sh, "-n", filepath.Join(scriptsDir(t), n)).CombinedOutput(); err != nil {
			t.Errorf("sh -n %s: %v\n%s", n, err, out)
		}
	}
}

// The real repo: workflows.conf is well formed, every script is present with LF endings, and once
// the reconciled skill set has landed every listed skill exists.
func TestRealRepoConsistency(t *testing.T) {
	dir := scriptsDir(t)
	conf := readConf(t, filepath.Join(dir, "workflows.conf"))
	name := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	for _, wf := range []string{"orchestration", "multi-agent"} {
		if len(conf[wf]) == 0 {
			t.Fatalf("workflows.conf lists no skills for %s", wf)
		}
		for _, s := range conf[wf] {
			if !name.MatchString(s) {
				t.Errorf("%s: bad skill name %q", wf, s)
			}
		}
	}
	for _, n := range []string{"workflows.conf", "apex-install-common.sh", "install.sh", "uninstall.sh", "apex-install-common.ps1", "install.ps1", "uninstall.ps1"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Errorf("missing scripts/%s", n)
			continue
		}
		if strings.Contains(string(b), "\r") {
			t.Errorf("scripts/%s has CR line endings", n)
		}
	}
	if !exists(filepath.Join(dir, "..", "skills", "orchestrator-protocol", "SKILL.md")) {
		t.Log("skills/orchestrator-protocol is not merged yet; skipping the skill existence check")
		return
	}
	for wf, skills := range conf {
		for _, s := range skills {
			if !exists(filepath.Join(dir, "..", "skills", s, "SKILL.md")) {
				t.Errorf("%s lists skills/%s/SKILL.md, which does not exist", wf, s)
			}
		}
	}
}
