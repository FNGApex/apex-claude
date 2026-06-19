package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"apexclaude/internal/handoff"
)

// initTestGitRepo creates a minimal git repo with one commit in dir.
// Returns false if git is not available; caller should t.Skip.
func initTestGitRepo(t *testing.T, dir string) bool {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+t.TempDir(),
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")
	return true
}

// makeTestCommit adds another commit so HEAD advances.
func makeTestCommit(t *testing.T, dir string) {
	t.Helper()
	f := filepath.Join(dir, "extra.md")
	if err := os.WriteFile(f, []byte("extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+t.TempDir(),
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "extra.md")
	run("commit", "-m", "second commit")
}

func TestHandoffStatusAbsent(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APEX_REPO", root)

	code := runHandoff([]string{"status"})
	if code != 1 {
		t.Errorf("status absent: got %d want 1", code)
	}
}

func TestHandoffScanCreatesFile(t *testing.T) {
	root := t.TempDir()
	if !initTestGitRepo(t, root) {
		t.Skip("git not available")
	}
	t.Setenv("APEX_REPO", root)

	code := runHandoff([]string{"scan"})
	if code != 0 {
		t.Fatalf("scan returned %d want 0", code)
	}

	// file must exist at Path
	p := handoff.Path(root)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("handoff file missing after scan: %v", err)
	}
}

func TestHandoffStatusFreshAfterScan(t *testing.T) {
	root := t.TempDir()
	if !initTestGitRepo(t, root) {
		t.Skip("git not available")
	}
	t.Setenv("APEX_REPO", root)

	if code := runHandoff([]string{"scan"}); code != 0 {
		t.Fatalf("scan returned %d", code)
	}

	code := runHandoff([]string{"status"})
	if code != 0 {
		t.Errorf("status fresh: got %d want 0", code)
	}
}

func TestHandoffStatusStaleAfterCommit(t *testing.T) {
	root := t.TempDir()
	if !initTestGitRepo(t, root) {
		t.Skip("git not available")
	}
	t.Setenv("APEX_REPO", root)

	if code := runHandoff([]string{"scan"}); code != 0 {
		t.Fatalf("scan returned %d", code)
	}

	makeTestCommit(t, root)

	code := runHandoff([]string{"status"})
	if code != 2 {
		t.Errorf("status stale: got %d want 2", code)
	}
}

func TestHandoffArchive(t *testing.T) {
	root := t.TempDir()
	if !initTestGitRepo(t, root) {
		t.Skip("git not available")
	}
	t.Setenv("APEX_REPO", root)

	if code := runHandoff([]string{"scan"}); code != 0 {
		t.Fatalf("scan returned %d", code)
	}

	code := runHandoff([]string{"archive"})
	if code != 0 {
		t.Errorf("archive returned %d want 0", code)
	}

	// active doc must be gone
	if _, err := os.Stat(handoff.Path(root)); !os.IsNotExist(err) {
		t.Error("active doc should be removed after archive")
	}

	// archived file must exist
	archiveDir := filepath.Join(root, ".claude", "project", "handoffs")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("handoffs dir missing: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 archived file, got %d", len(entries))
	}
}

func TestHandoffArchiveNothingToArchive(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APEX_REPO", root)

	// no active doc — archive should fail with exit 1
	code := runHandoff([]string{"archive"})
	if code != 1 {
		t.Errorf("archive with nothing: got %d want 1", code)
	}
}

func TestHandoffScanWithMode(t *testing.T) {
	root := t.TempDir()
	if !initTestGitRepo(t, root) {
		t.Skip("git not available")
	}
	t.Setenv("APEX_REPO", root)

	code := runHandoff([]string{"scan", "urgent"})
	if code != 0 {
		t.Fatalf("scan urgent returned %d want 0", code)
	}

	p := handoff.Path(root)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("handoff file missing after scan urgent: %v", err)
	}
}

func TestHandoffScanInvalidMode(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APEX_REPO", root)

	code := runHandoff([]string{"scan", "badmode"})
	if code != 2 {
		t.Errorf("scan badmode: got %d want 2", code)
	}
}

func TestHandoffUnknownSubcommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APEX_REPO", root)

	code := runHandoff([]string{"bogus"})
	if code != 2 {
		t.Errorf("unknown sub: got %d want 2", code)
	}
}

func TestHandoffNoSubcommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APEX_REPO", root)

	code := runHandoff([]string{})
	if code != 2 {
		t.Errorf("empty args: got %d want 2", code)
	}
}
