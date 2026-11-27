package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setup(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(root, ".claude", "skills", "dev")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: dev\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func apex(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(dir, args, &out, &errb)
	return code, out.String(), errb.String()
}

// The model writes flags after positionals; stdlib flag parsing alone would silently drop them.
func TestFlagsAfterPositionals(t *testing.T) {
	root := setup(t)
	code, out, errs := apex(t, root, "add", "task", "wire the hook", "--agent", "dev")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	want := filepath.Join(root, ".claude", "ledger", "dev", "001.md")
	if strings.TrimSpace(out) != want {
		t.Fatalf("add printed %q, want %q", out, want)
	}
}

func TestExitCodes(t *testing.T) {
	root := setup(t)
	apex(t, root, "add", "task", "a", "--agent", "dev")
	cases := []struct {
		args []string
		code int
	}{
		{[]string{}, 64},
		{[]string{"frobnicate"}, 64},
		{[]string{"list"}, 64}, // no default agent
		{[]string{"list", "--agent", "dev", "--bogus"}, 64},
		{[]string{"start", "../x", "--agent", "dev"}, 64},
		{[]string{"start", "9", "--agent", "dev"}, 1},
		{[]string{"list", "--agent", "ghost"}, 1},
		{[]string{"start", "1", "--agent", "dev"}, 0},
		{[]string{"start", "1", "--agent", "dev"}, 3},
		{[]string{"close", "1", "--agent", "dev"}, 64}, // reason required
		{[]string{"close", "1", "shipped", "--agent", "dev"}, 0},
		{[]string{"reopen", "1", "--agent", "dev"}, 0},
	}
	for _, c := range cases {
		if code, _, errs := apex(t, root, c.args...); code != c.code {
			t.Errorf("apex %v: exit %d, want %d (%s)", c.args, code, c.code, strings.TrimSpace(errs))
		}
	}
}

func TestListFiltersAndCounts(t *testing.T) {
	root := setup(t)
	apex(t, root, "add", "task", "alpha", "--agent", "dev")
	apex(t, root, "add", "followup", "beta", "--agent", "dev")
	apex(t, root, "close", "1", "done", "--agent", "dev")

	_, out, _ := apex(t, root, "list", "--agent", "dev")
	if strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("default list must hide closed items:\n%s", out)
	}
	if !strings.Contains(out, "1 open, 0 in-progress, 1 closed") {
		t.Fatalf("missing counts line:\n%s", out)
	}
	_, out, _ = apex(t, root, "list", "--agent", "dev", "--status", "all", "--kind", "task")
	if !strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Fatalf("--status all --kind task:\n%s", out)
	}
}

// Every Agent's state lives in the main checkout, so a call from a subdirectory must reach the same ledger.
func TestRunFromSubdirUsesMainCheckout(t *testing.T) {
	root := setup(t)
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	sub := filepath.Join(root, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := apex(t, root, "add", "task", "a", "--agent", "dev"); code != 0 {
		t.Fatal(errs)
	}
	_, out, _ := apex(t, sub, "list", "--agent", "dev")
	if !strings.Contains(out, "001") {
		t.Fatalf("list from subdir missed the main checkout's ledger:\n%s", out)
	}
}

// Usage is judged before the disk is read, so the same bad call exits 64 whatever the ledger holds.
func TestUsageBeatsDiskState(t *testing.T) {
	root := setup(t)
	apex(t, root, "add", "task", "a", "--agent", "dev")
	if err := os.WriteFile(filepath.Join(root, ".claude", "ledger", "dev", "002.md"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"start", "abc", "--agent", "ghost"},
		{"retitle", "1", "a\nb", "--agent", "ghost"},
		{"add", "bogus", "t", "--agent", "ghost"},
		{"list", "--status", "bogus", "--agent", "dev"},
		{"list", "--kind", "taks", "--agent", "dev"},
		{"list", "--title", "x", "--agent", "dev"},
		{"start", "1", "--status", "open", "--agent", "dev"},
		{"close", "1", "a\nb", "--agent", "dev"},
	} {
		if code, _, errs := apex(t, root, args...); code != 64 {
			t.Errorf("apex %q: exit %d, want 64 (%s)", args, code, strings.TrimSpace(errs))
		}
	}
	if code, _, _ := apex(t, root, "list", "--agent", "dev"); code != 4 {
		t.Errorf("list over a corrupt item: exit %d, want 4", code)
	}
}

func TestEnvironmentErrorExits2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0 does not deny reads on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := setup(t)
	apex(t, root, "add", "task", "a", "--agent", "dev")
	dir := filepath.Join(root, ".claude", "ledger", "dev")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if code, _, _ := apex(t, root, "list", "--agent", "dev"); code != 2 {
		t.Errorf("unreadable ledger: exit %d, want 2", code)
	}
}

func TestDashTitleAfterDoubleDash(t *testing.T) {
	root := setup(t)
	if code, _, errs := apex(t, root, "add", "task", "--agent", "dev", "--", "-starts with dash"); code != 0 {
		t.Fatalf("title after --: exit %d (%s)", code, errs)
	}
	if _, out, _ := apex(t, root, "list", "--agent", "dev"); !strings.Contains(out, "-starts with dash") {
		t.Fatalf("dash title missing:\n%s", out)
	}
}

func TestHelpInsideVerb(t *testing.T) {
	root := setup(t)
	if code, out, _ := apex(t, root, "list", "-h"); code != 0 || !strings.Contains(out, "usage:") {
		t.Fatalf("list -h: exit %d, out %q", code, out)
	}
}

func TestCountsFollowKindFilter(t *testing.T) {
	root := setup(t)
	apex(t, root, "add", "task", "a", "--agent", "dev")
	apex(t, root, "add", "followup", "b", "--agent", "dev")
	if _, out, _ := apex(t, root, "list", "--agent", "dev", "--kind", "followup"); !strings.Contains(out, "1 open, 0 in-progress, 0 closed") {
		t.Fatalf("counts ignore --kind:\n%s", out)
	}
}

// Where the main checkout cannot be resolved, only the check before repo.Main turns a bad agent name into 64.
func TestBadAgentIsUsageEvenWhenGitFails(t *testing.T) {
	r, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "i"}, {"worktree", "add", "-q", "-b", "f", wt}} {
		if out, err := exec.Command("git", append([]string{"-C", r}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.Rename(r, r+"-moved"); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := apex(t, wt, "start", "1", "--agent", "Dev"); code != 64 {
		t.Fatalf("exit %d, want 64 (%s)", code, errs)
	}
}

func TestRetitleVerb(t *testing.T) {
	root := setup(t)
	apex(t, root, "add", "task", "old", "--agent", "dev")
	if code, _, errs := apex(t, root, "retitle", "1", "new title", "--agent", "dev"); code != 0 {
		t.Fatalf("retitle: exit %d (%s)", code, errs)
	}
	if _, out, _ := apex(t, root, "list", "--agent", "dev"); !strings.Contains(out, "new title") {
		t.Fatalf("list after retitle:\n%s", out)
	}
	if code, _, _ := apex(t, root, "retitle", "1", "--agent", "dev"); code != 64 {
		t.Fatalf("retitle without title: exit %d, want 64", code)
	}
}

// A per-ticket Agent keeps a separate ledger per branch, numbered from 001; without the marker it has one mono ledger.
func TestPerTicketLedger(t *testing.T) {
	root := setup(t)
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "feat/eng-42-login")
	git("commit", "-q", "--allow-empty", "-m", "init")
	dev := filepath.Join(root, ".claude", "ledger", "dev")

	_, out, _ := apex(t, root, "add", "task", "mono", "--agent", "dev")
	if strings.TrimSpace(out) != filepath.Join(dev, "001.md") {
		t.Fatalf("mono add printed %q", out)
	}
	if err := os.WriteFile(filepath.Join(dev, "per-ticket"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, out, _ = apex(t, root, "add", "task", "login", "--agent", "dev")
	if strings.TrimSpace(out) != filepath.Join(dev, "ENG-42", "001.md") {
		t.Fatalf("ticket add printed %q", out)
	}
	git("checkout", "-q", "-b", "spike")
	_, out, _ = apex(t, root, "list", "--agent", "dev")
	if want := "ledger: " + filepath.Join(dev, "spike") + "\n0 open, 0 in-progress, 0 closed\n"; out != want {
		t.Fatalf("list on spike = %q, want %q", out, want)
	}
	git("checkout", "-q", "--detach")
	_, out, _ = apex(t, root, "list", "--agent", "dev")
	if !strings.Contains(out, "mono") || strings.Contains(out, "ledger:") {
		t.Fatalf("detached list = %q, want the mono ledger", out)
	}
}

func TestWhere(t *testing.T) {
	root := setup(t)
	code, out, errs := apex(t, root, "where", "--agent", "dev", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json %v:\n%s", err, out)
	}
	for k, want := range map[string]any{
		"layout": "legacy", "ledger": filepath.Join(root, ".claude", "ledger", "dev"),
		"private": filepath.Join(root, ".claude", "apex", "dev"), "framing": filepath.Join(root, "CLAUDE.md"),
		"workers": filepath.Join(root, ".apex", "workers"), "per_ticket": false, "concurrent": false,
	} {
		if got[k] != want {
			t.Errorf("%s = %v, want %v", k, got[k], want)
		}
	}
	if code, out, _ := apex(t, root, "where"); code != 0 || !strings.Contains(out, "layout: apex") || strings.Contains(out, "ledger:") {
		t.Errorf("where without agent on a fresh repo: exit %d\n%s", code, out)
	}
	for _, args := range [][]string{{"where", "--agent", "Dev"}, {"where", "x"}, {"where", "--bogus"}} {
		if code, _, _ := apex(t, root, args...); code != 64 {
			t.Errorf("apex %v: exit %d, want 64", args, code)
		}
	}
	if code, _, _ := apex(t, root, "where", "--agent", "ghost"); code != 1 {
		t.Errorf("where ghost: exit %d, want 1", code)
	}
}

// Usage is judged before git is asked anything, so these exit 64 even outside a repository.
func TestWorkerAndSkillUsage(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"worker"}, {"worker", "frob"}, {"worker", "spawn"}, {"worker", "spawn", "Bad_Slug"},
		{"worker", "spawn", "ok", "--skill", "a,,b"}, {"worker", "spawn", "ok", "--item", "../1"},
		{"worker", "list", "--state", "zombie"}, {"worker", "status", "abc"}, {"worker", "merge", "0"},
		{"worker", "remove", "1", "2"}, {"worker", "prune", "x"},
		{"worker", "adopt"}, {"worker", "adopt", "a", "b"}, {"worker", "adopt", ""}, {"worker", "adopt", "x\ty"},
		{"worker", "adopt", "t3/x", "--slug", "Bad"}, {"worker", "adopt", "t3/x", "--skill", "a,,b"},
		{"worker", "adopt", "t3/x", "--item", "../1"}, {"worker", "remove", "1", "--purge=maybe"},
		{"skill"}, {"skill", "frob"}, {"skill", "search"}, {"skill", "show"}, {"skill", "list", "--kind", "wizard"},
		{"skill", "list", "--source", "web"}, {"skill", "search", "x", "--limit", "0"}, {"skill", "check", "a", "b"},
	} {
		if code, _, errs := apex(t, dir, args...); code != 64 {
			t.Errorf("apex %q: exit %d, want 64 (%s)", args, code, strings.TrimSpace(errs))
		}
	}
}

func TestWorkerVerbs(t *testing.T) {
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := setup(t)
	git := func(dir string, args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(root, "init", "-q", "-b", "main")
	git(root, "config", "core.autocrlf", "false")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\n"), 0o644)
	git(root, "add", "a.txt")
	git(root, "commit", "-q", "-m", "init")

	code, out, errs := apex(t, root, "worker", "spawn", "clash", "--skill", "go", "--json")
	if code != 0 {
		t.Fatalf("spawn: exit %d: %s", code, errs)
	}
	var v struct {
		ID, Branch, Worktree string
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.ID != "001" || v.Branch != "apex/001-clash" {
		t.Fatalf("spawn json %v:\n%s", err, out)
	}
	os.WriteFile(filepath.Join(v.Worktree, "a.txt"), []byte("worker\n"), 0o644)
	git(v.Worktree, "commit", "-q", "-am", "w")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("main\n"), 0o644)
	git(root, "commit", "-q", "-am", "m")

	if _, out, _ := apex(t, root, "worker", "list"); !strings.Contains(out, "ahead 1  behind 1") || !strings.Contains(out, "1 active, 0 merged, 0 removed") {
		t.Fatalf("list:\n%s", out)
	}
	code, out, _ = apex(t, root, "worker", "merge", "1", "--json")
	if code != 5 || !strings.Contains(out, `"a.txt"`) {
		t.Fatalf("conflicting merge: exit %d, want 5\n%s", code, out)
	}
	if code, _, _ := apex(t, root, "worker", "status", "9"); code != 1 {
		t.Fatalf("status of missing worker: exit %d, want 1", code)
	}
	if code, _, _ := apex(t, root, "worker", "remove", "1"); code != 3 {
		t.Fatalf("remove unmerged: exit %d, want 3", code)
	}
	if code, out, errs := apex(t, root, "worker", "remove", "1", "--force"); code != 0 || out != "removed 001\n" {
		t.Fatalf("force remove: exit %d %q %s", code, out, errs)
	}
	if _, out, _ := apex(t, root, "worker", "list", "--state", "all"); !strings.Contains(out, "0 active, 0 merged, 1 removed") {
		t.Fatalf("list all:\n%s", out)
	}
}

func TestSkillVerbs(t *testing.T) {
	root := setup(t)
	t.Setenv("APEX_SKILLS_DIR", "")
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	if code, out, errs := apex(t, root, "skill", "list"); code != 0 || !strings.Contains(out, "dev") || !strings.Contains(out, "1 skills") {
		t.Fatalf("list: exit %d %s\n%s", code, errs, out)
	}
	// setup's SKILL.md has no description: check must fail with 4 and name the problem.
	if code, out, _ := apex(t, root, "skill", "check"); code != 4 || !strings.Contains(out, "description is missing") {
		t.Fatalf("check: exit %d\n%s", code, out)
	}
	if code, _, _ := apex(t, root, "skill", "show", "ghost"); code != 1 {
		t.Fatalf("show ghost: exit %d, want 1", code)
	}
	if code, out, _ := apex(t, root, "skill", "show", "dev", "--body"); code != 0 || !strings.Contains(out, "name: dev\n---") {
		t.Fatalf("show --body: exit %d\n%s", code, out)
	}
	if code, out, _ := apex(t, root, "skill", "search", "dev", "--json"); code != 0 || !strings.Contains(out, `"score": 8`) {
		t.Fatalf("search: exit %d\n%s", code, out)
	}
	shipped, err := filepath.Abs(filepath.Join("..", "..", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if code, out, errs := apex(t, root, "skill", "check", shipped); code != 0 || !strings.Contains(out, " 0 problems") {
		t.Fatalf("check of the shipped skills: exit %d %s\n%s", code, errs, out)
	}
	if code, _, _ := apex(t, root, "skill", "check", "ghost"); code != 1 {
		t.Fatalf("check of an unknown name: exit %d, want 1", code)
	}
	longDir := filepath.Join(root, ".agents", "skills", "long-desc")
	if err := os.MkdirAll(longDir, 0o755); err != nil {
		t.Fatal(err)
	}
	longDesc := strings.Repeat("x", 100)
	if err := os.WriteFile(filepath.Join(longDir, "SKILL.md"), []byte("---\nname: long-desc\ndescription: "+longDesc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, out, _ := apex(t, root, "skill", "list"); !strings.Contains(out, strings.Repeat("x", 77)+"...") || strings.Contains(out, longDesc) {
		t.Fatalf("long description must truncate in human list:\n%s", out)
	}
	if _, out, _ := apex(t, root, "skill", "show", "long-desc"); !strings.Contains(out, longDesc) {
		t.Fatalf("show must keep full description:\n%s", out)
	}
}

func TestWorkerAdopt(t *testing.T) {
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := setup(t)
	ext, err := filepath.EvalSymlinks(t.TempDir()) // stands in for ~/.t3/worktrees/...
	if err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(root, "init", "-q", "-b", "main")
	git(root, "config", "core.autocrlf", "false")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\n"), 0o644)
	git(root, "add", "a.txt")
	git(root, "commit", "-q", "-m", "init")
	feat := filepath.Join(ext, "feat")
	git(root, "worktree", "add", "-q", "-b", "t3/feat-x", feat)
	git(root, "worktree", "add", "-q", "--detach", filepath.Join(ext, "det"))
	git(root, "branch", "t3/solo")

	for _, c := range []struct {
		args []string
		code int
	}{
		{[]string{root}, 3}, {[]string{"main"}, 3}, {[]string{filepath.Join(ext, "det")}, 3}, {[]string{"nope"}, 1},
		{[]string{"t3/solo", "--base", "t3/solo"}, 3},
	} {
		if code, _, errs := apex(t, root, append([]string{"worker", "adopt"}, c.args...)...); code != c.code {
			t.Errorf("adopt %q: exit %d, want %d (%s)", c.args, code, c.code, strings.TrimSpace(errs))
		}
	}

	code, out, errs := apex(t, root, "worker", "adopt", feat, "--skill", "go", "--item", "7", "--json")
	if code != 0 {
		t.Fatalf("adopt: exit %d: %s", code, errs)
	}
	var v struct {
		ID, Slug, Branch, Worktree string
		External                   bool
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.ID != "001" || v.Branch != "t3/feat-x" ||
		v.Slug != "feat-x" || !v.External || !strings.EqualFold(filepath.Clean(v.Worktree), feat) {
		t.Fatalf("adopt json %v:\n%s", err, out)
	}
	if code, _, _ := apex(t, root, "worker", "adopt", "t3/feat-x"); code != 3 {
		t.Fatalf("duplicate adopt: exit %d, want 3", code)
	}
	if code, out, _ := apex(t, root, "worker", "adopt", "t3/solo"); code != 0 || !strings.Contains(out, "worktree: (branch only)") {
		t.Fatalf("branch adopt: exit %d\n%s", code, out)
	}
	if _, out, _ := apex(t, root, "worker", "status", "2"); !strings.Contains(out, "live: branch") {
		t.Fatalf("branch status:\n%s", out)
	}

	os.WriteFile(filepath.Join(feat, "b.txt"), []byte("feat\n"), 0o644)
	git(feat, "add", "b.txt")
	git(feat, "commit", "-q", "-m", "feat")
	if code, _, errs := apex(t, root, "worker", "merge", "1"); code != 0 {
		t.Fatalf("merge: exit %d: %s", code, errs)
	}
	if _, err := os.Stat(filepath.Join(root, "b.txt")); err != nil {
		t.Fatalf("merge did not land on main: %v", err)
	}
	if code, out, _ := apex(t, root, "worker", "prune"); code != 0 || !strings.Contains(out, "removed 001") {
		t.Fatalf("prune: exit %d\n%s", code, out)
	}
	if _, err := os.Stat(feat); err != nil || git(root, "branch", "--list", "t3/feat-x") == "" {
		t.Fatalf("prune deleted the adopted worktree or branch: %v", err)
	}
	if code, _, _ := apex(t, root, "worker", "remove", "2"); code != 0 || git(root, "branch", "--list", "t3/solo") == "" {
		t.Fatalf("remove: exit %d or branch deleted", code)
	}
	if code, _, errs := apex(t, root, "worker", "adopt", "t3/feat-x"); code != 0 {
		t.Fatalf("re-adopt: exit %d: %s", code, errs)
	}
	if code, _, errs := apex(t, root, "worker", "remove", "3", "--purge"); code != 0 {
		t.Fatalf("purge: exit %d: %s", code, errs)
	}
	if _, err := os.Stat(feat); !os.IsNotExist(err) || git(root, "branch", "--list", "t3/feat-x") != "" {
		t.Fatalf("purge left worktree or branch: %v", err)
	}
}
