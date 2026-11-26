package worker

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"apex/internal/exit"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo makes a main checkout on branch main with one commit, and sets the identity merges need.
func newRepo(t *testing.T) (string, *Manager) {
	t.Helper()
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t",
		"GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run(t, root, "init", "-q", "-b", "main")
	run(t, root, "config", "core.autocrlf", "false")
	write(t, filepath.Join(root, "a.txt"), "one\n")
	run(t, root, "add", "a.txt")
	run(t, root, "commit", "-q", "-m", "init")
	return root, New(root)
}

func commit(t *testing.T, dir, file, data string) {
	t.Helper()
	write(t, filepath.Join(dir, file), data)
	run(t, dir, "add", file)
	run(t, dir, "commit", "-q", "-m", "edit "+file)
}

func TestValidation(t *testing.T) {
	for _, s := range []string{"", "-x", "Up", "a_b", strings.Repeat("a", 41)} {
		if CheckSlug(s) == nil {
			t.Errorf("CheckSlug(%q) accepted", s)
		}
	}
	for _, s := range []string{"x", "../1", "Dev/1", "dev/"} {
		if CheckItem(s) == nil {
			t.Errorf("CheckItem(%q) accepted", s)
		}
	}
	for _, s := range []string{"", "7", "orch/7"} {
		if err := CheckItem(s); err != nil {
			t.Errorf("CheckItem(%q): %v", s, err)
		}
	}
	if got, err := ParseSkills(" a, b-2 "); err != nil || strings.Join(got, ",") != "a,b-2" {
		t.Errorf("ParseSkills = %v, %v", got, err)
	}
	if _, err := ParseSkills("a,,b"); !errors.Is(err, exit.Usage) {
		t.Errorf("empty skill: err = %v, want Usage", err)
	}
}

func TestLifecycle(t *testing.T) {
	root, m := newRepo(t)
	mf, err := m.Spawn(root, "fix-login", "", []string{"go"}, "orch/3")
	if err != nil {
		t.Fatal(err)
	}
	if mf.ID != "001" || mf.Branch != "apex/001-fix-login" || mf.Base != "main" || mf.State != StateActive {
		t.Fatalf("spawned %+v", mf)
	}
	if data, err := os.ReadFile(filepath.Join(m.dir, ".gitignore")); err != nil || string(data) != "*\n" {
		t.Fatalf(".gitignore = %q, %v", data, err)
	}
	if out := run(t, root, "status", "--porcelain"); out != "" {
		t.Fatalf("main checkout dirty after spawn:\n%s", out)
	}
	wt := m.Worktree(mf.ID)

	// Nothing to merge yet.
	if _, err := m.Merge(1, ""); !errors.Is(err, exit.Transition) {
		t.Fatalf("merge with no commits: err = %v, want Transition", err)
	}
	commit(t, wt, "b.txt", "worker\n")
	write(t, filepath.Join(wt, "scratch"), "x")
	lv, err := m.Live(mf)
	if err != nil || lv.Worktree != "ok" || lv.Dirty != 1 || lv.Ahead != 1 || lv.Behind != 0 || lv.Head == "" {
		t.Fatalf("live = %+v, %v", lv, err)
	}
	if _, err := m.Merge(1, ""); !errors.Is(err, exit.Transition) {
		t.Fatalf("merge of dirty worker: err = %v, want Transition", err)
	}
	if _, err := m.Remove(1, false); !errors.Is(err, exit.Transition) {
		t.Fatalf("remove of dirty worker: err = %v, want Transition", err)
	}
	os.Remove(filepath.Join(wt, "scratch"))

	// Staged edits in the base checkout block the merge.
	write(t, filepath.Join(root, "a.txt"), "staged\n")
	run(t, root, "add", "a.txt")
	if _, err := m.Merge(1, ""); !errors.Is(err, exit.Transition) {
		t.Fatalf("merge over staged base: err = %v, want Transition", err)
	}
	run(t, root, "reset", "-q", "--hard")

	mf, err = m.Merge(1, "")
	if err != nil {
		t.Fatal(err)
	}
	if mf.State != StateMerged || mf.MergedSHA != run(t, root, "rev-parse", "HEAD") || mf.Merged == "" {
		t.Fatalf("merged %+v", mf)
	}
	if msg := run(t, root, "log", "-1", "--format=%s"); msg != "Merge worker 001 (fix-login) into main" {
		t.Fatalf("merge message %q", msg)
	}

	// A second, unmerged worker survives prune; the merged one goes.
	other, err := m.Spawn(root, "spike", "main", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	commit(t, m.Worktree(other.ID), "c.txt", "spike\n")
	removed, skipped, err := m.Prune()
	if err != nil || len(removed) != 1 || removed[0].ID != "001" || len(skipped) != 0 {
		t.Fatalf("prune = %v, %v, %v", removed, skipped, err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree still there: %v", err)
	}
	if got, _ := m.Get(1); got.State != StateRemoved || got.Removed == "" {
		t.Fatalf("after prune %+v", got)
	}
	if _, err := m.Remove(1, false); !errors.Is(err, exit.Transition) {
		t.Fatalf("remove twice: err = %v, want Transition", err)
	}
	if _, err := m.Remove(2, false); !errors.Is(err, exit.Transition) {
		t.Fatalf("remove unmerged: err = %v, want Transition", err)
	}
	if got, err := m.Remove(2, true); err != nil || got.State != StateRemoved {
		t.Fatalf("force remove = %+v, %v", got, err)
	}
	if out := run(t, root, "branch", "--list", "apex/*"); out != "" {
		t.Fatalf("worker branches left: %s", out)
	}
	if all, err := m.List(); err != nil || len(all) != 2 {
		t.Fatalf("List = %v, %v", all, err)
	}
}

// A conflicting merge is aborted, leaves the base checkout as it was, and names the conflicting files.
func TestMergeConflict(t *testing.T) {
	root, m := newRepo(t)
	mf, err := m.Spawn(root, "clash", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	commit(t, m.Worktree(mf.ID), "a.txt", "worker\n")
	commit(t, root, "a.txt", "main\n")
	before := run(t, root, "rev-parse", "HEAD")
	_, err = m.Merge(1, "")
	var ce *ConflictError
	if !errors.Is(err, exit.Conflict) || !errors.As(err, &ce) || strings.Join(ce.Files, ",") != "a.txt" {
		t.Fatalf("err = %v, want a conflict on a.txt", err)
	}
	if exit.Code(err) != 5 {
		t.Fatalf("exit code %d, want 5", exit.Code(err))
	}
	if run(t, root, "rev-parse", "HEAD") != before || run(t, root, "status", "--porcelain") != "" {
		t.Fatal("conflicting merge was not aborted cleanly")
	}
	if got, _ := m.Get(1); got.State != StateActive {
		t.Fatalf("state after conflict = %s", got.State)
	}
}

func TestMergeNeedsBaseCheckedOut(t *testing.T) {
	root, m := newRepo(t)
	if _, err := m.Spawn(root, "x", "", nil, ""); err != nil {
		t.Fatal(err)
	}
	commit(t, m.Worktree("001"), "b.txt", "x\n")
	run(t, root, "checkout", "-q", "--detach")
	if _, err := m.Merge(1, ""); !errors.Is(err, exit.Transition) || !strings.Contains(err.Error(), "check out main") {
		t.Fatalf("err = %v, want Transition naming main", err)
	}
}

func TestSpawnFailures(t *testing.T) {
	root, m := newRepo(t)
	if _, err := m.Spawn(root, "x", "nope", nil, ""); !errors.Is(err, exit.NotFound) {
		t.Fatalf("unknown base: err = %v, want NotFound", err)
	}
	run(t, root, "checkout", "-q", "--detach")
	if _, err := m.Spawn(root, "x", "", nil, ""); err == nil || exit.Code(err) != 2 {
		t.Fatalf("detached without --base: err = %v, want environment", err)
	}
	// A failing git worktree add leaves no manifest behind.
	run(t, root, "branch", "apex/001-x")
	if _, err := m.Spawn(root, "x", "main", nil, ""); err == nil || exit.Code(err) != 2 {
		t.Fatalf("taken branch: err = %v, want environment", err)
	}
	if all, err := m.List(); err != nil || len(all) != 0 {
		t.Fatalf("manifests after failed spawn: %v, %v", all, err)
	}
}

func TestCorruptManifest(t *testing.T) {
	root, m := newRepo(t)
	if _, err := m.Spawn(root, "x", "", nil, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.dir, "001.json")
	good, _ := os.ReadFile(path)
	for name, data := range map[string]string{
		"bad json":      "{",
		"unknown field": strings.Replace(string(good), `"id"`, `"bogus": 1, "id"`, 1),
		"id mismatch":   strings.Replace(string(good), `"id": "001"`, `"id": "002"`, 1),
		"bad state":     strings.Replace(string(good), `"active"`, `"zombie"`, 1),
		"bad branch":    strings.Replace(string(good), `"apex/001-x"`, `"main"`, 1),
	} {
		write(t, path, data)
		if _, err := m.List(); !errors.Is(err, exit.Corrupt) {
			t.Errorf("%s: err = %v, want Corrupt", name, err)
		}
	}
	if _, err := m.Get(9); !errors.Is(err, exit.NotFound) {
		t.Errorf("missing: err = %v, want NotFound", err)
	}
}
