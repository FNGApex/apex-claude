package handoff

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"apexclaude/internal/fm"
)

// initGitRepo creates a minimal git repo with one commit in dir.
// Returns false if git is not available, caller should t.Skip.
func initGitRepo(t *testing.T, dir string) bool {
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
			"HOME="+t.TempDir(), // isolate global config
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	// write a file so we can commit
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")
	return true
}

// makeCommit adds another commit to the repo so HEAD advances.
func makeCommit(t *testing.T, dir string) {
	t.Helper()
	f := filepath.Join(dir, "extra.md")
	if err := os.WriteFile(f, []byte("extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "extra.md")
	run("commit", "-m", "second commit")
}

// --- Checkpoint 1 & 2: Scan ---

func TestScanGitFields(t *testing.T) {
	root := t.TempDir()
	if !initGitRepo(t, root) {
		t.Skip("git not available")
	}

	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if s.Branch == "" {
		t.Error("Branch should be non-empty in a git repo")
	}
	if s.Head == "" {
		t.Error("Head should be non-empty in a git repo")
	}
	if len(s.Head) > 12 {
		t.Errorf("Head should be short sha, got %q (len %d)", s.Head, len(s.Head))
	}
	if s.LastCommit == "" {
		t.Error("LastCommit should be non-empty")
	}
}

func TestScanNonRepo(t *testing.T) {
	root := t.TempDir()
	// no git init — should not error
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan outside repo should not error, got: %v", err)
	}
	// git fields should be zero values
	if s.Branch != "" || s.Head != "" {
		t.Errorf("expected zero git fields outside repo, got branch=%q head=%q", s.Branch, s.Head)
	}
}

func TestScanNonGitFields(t *testing.T) {
	root := t.TempDir()
	// No followups, reminders, health file → zero/defaults expected
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// OpenFollowups should be 0 when none exist
	if s.OpenFollowups != 0 {
		t.Errorf("expected OpenFollowups=0, got %d", s.OpenFollowups)
	}
	// DueReminders should be 0 when none exist
	if s.DueReminders != 0 {
		t.Errorf("expected DueReminders=0, got %d", s.DueReminders)
	}
	// Health should be -1 when file absent
	if s.Health != -1 {
		t.Errorf("expected Health=-1, got %d", s.Health)
	}
}

// --- Checkpoint 3: Render ---

func TestRenderGracefulSections(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	s := State{Branch: "main", Head: "abc1234", Health: 80}
	out := Render(s, "graceful", now)

	for _, section := range []string{"## Shipped", "## Outcome", "## Next", "## Open threads"} {
		if !strings.Contains(out, section) {
			t.Errorf("graceful render missing section %q", section)
		}
	}
	// urgent sections must NOT appear
	for _, section := range []string{"## Cursor", "## Uncommitted", "## Resume here", "## Blockers"} {
		if strings.Contains(out, section) {
			t.Errorf("graceful render should not contain urgent section %q", section)
		}
	}
}

func TestRenderUrgentSections(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	s := State{Branch: "main", Head: "abc1234", Health: 60}
	out := Render(s, "urgent", now)

	for _, section := range []string{"## Cursor", "## Uncommitted", "## Resume here", "## Blockers"} {
		if !strings.Contains(out, section) {
			t.Errorf("urgent render missing section %q", section)
		}
	}
	// graceful sections must NOT appear
	for _, section := range []string{"## Shipped", "## Outcome", "## Next", "## Open threads"} {
		if strings.Contains(out, section) {
			t.Errorf("urgent render should not contain graceful section %q", section)
		}
	}
}

func TestRenderFrontmatterRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	s := State{Branch: "feat/x", Head: "deadbee", Health: 75}
	out := Render(s, "graceful", now)

	meta, _ := fm.Parse(out)
	if meta["mode"] != "graceful" {
		t.Errorf("mode: got %q", meta["mode"])
	}
	if meta["branch"] != "feat/x" {
		t.Errorf("branch: got %q", meta["branch"])
	}
	if meta["head"] != "deadbee" {
		t.Errorf("head: got %q", meta["head"])
	}
	if meta["health"] != "75" {
		t.Errorf("health: got %q", meta["health"])
	}
	if meta["status"] != "open" {
		t.Errorf("status: got %q", meta["status"])
	}
	if meta["created"] == "" {
		t.Error("created must not be empty")
	}
}

func TestRenderFrontmatterKeyOrder(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	s := State{Branch: "main", Head: "abc1234", Health: 80}
	out := Render(s, "graceful", now)

	// Verify ordered keys appear in sequence in the raw string
	order := []string{"mode:", "created:", "branch:", "head:", "health:", "status:"}
	last := 0
	for _, key := range order {
		idx := strings.Index(out[last:], key)
		if idx < 0 {
			t.Errorf("key %q not found after position %d", key, last)
			break
		}
		last += idx + len(key)
	}
}

// --- Checkpoint 4: Path, Write, Status, Archive ---

func TestPath(t *testing.T) {
	root := t.TempDir()
	p := Path(root)
	expected := filepath.Join(root, ".claude", "project", "handoff.md")
	if p != expected {
		t.Errorf("Path=%q want %q", p, expected)
	}
}

func TestStatusAbsent(t *testing.T) {
	root := t.TempDir()
	code := Status(root)
	if code != 1 {
		t.Errorf("Status absent: got %d want 1", code)
	}
}

func TestStatusFresh(t *testing.T) {
	root := t.TempDir()
	if !initGitRepo(t, root) {
		t.Skip("git not available")
	}

	now := time.Now()
	s, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(root, s, "graceful", now); err != nil {
		t.Fatalf("Write: %v", err)
	}

	code := Status(root)
	if code != 0 {
		t.Errorf("Status fresh: got %d want 0", code)
	}
}

func TestStatusStale(t *testing.T) {
	root := t.TempDir()
	if !initGitRepo(t, root) {
		t.Skip("git not available")
	}

	now := time.Now()
	s, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(root, s, "graceful", now); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// advance HEAD
	makeCommit(t, root)

	code := Status(root)
	if code != 2 {
		t.Errorf("Status stale: got %d want 2", code)
	}
}

func TestArchive(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	s := State{Branch: "main", Head: "abc1234", Health: 80}

	if err := Write(root, s, "graceful", now); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// active doc should exist
	if _, err := os.Stat(Path(root)); err != nil {
		t.Fatalf("active doc missing after Write: %v", err)
	}

	id, err := Archive(root)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}

	// must be %03d
	if len(id) != 3 {
		t.Errorf("Archive id %q should be 3 chars", id)
	}

	// archived file must exist
	archivePath := filepath.Join(root, ".claude", "project", "handoffs", id+".md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archived file missing at %s: %v", archivePath, err)
	}

	// status must be consumed
	meta, _ := fm.Parse(string(data))
	if meta["status"] != "consumed" {
		t.Errorf("archived status: got %q want consumed", meta["status"])
	}

	// active doc must be gone
	if _, err := os.Stat(Path(root)); !os.IsNotExist(err) {
		t.Error("active doc should be removed after Archive")
	}
}

func TestArchiveNextID(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	s := State{Branch: "main", Head: "abc1234", Health: 80}

	// First archive
	if err := Write(root, s, "graceful", now); err != nil {
		t.Fatal(err)
	}
	id1, err := Archive(root)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != "001" {
		t.Errorf("first archive id: got %q want 001", id1)
	}

	// Second archive
	if err := Write(root, s, "urgent", now); err != nil {
		t.Fatal(err)
	}
	id2, err := Archive(root)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != "002" {
		t.Errorf("second archive id: got %q want 002", id2)
	}
}

func TestWriteArchivesExistingUnconsumed(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	s := State{Branch: "main", Head: "abc1234", Health: 80}

	// Write first doc
	if err := Write(root, s, "graceful", now); err != nil {
		t.Fatalf("first Write: %v", err)
	}

	// Write second doc — should archive the first
	s2 := State{Branch: "feat", Head: "bbb2222", Health: 90}
	if err := Write(root, s2, "urgent", now); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	// The archived copy must exist
	archivesDir := filepath.Join(root, ".claude", "project", "handoffs")
	entries, err := os.ReadDir(archivesDir)
	if err != nil {
		t.Fatalf("handoffs dir missing: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 archived file, got %d", len(entries))
	}

	// Active doc must be the new one (branch=feat)
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatalf("active doc missing: %v", err)
	}
	meta, _ := fm.Parse(string(data))
	if meta["branch"] != "feat" {
		t.Errorf("active doc branch: got %q want feat", meta["branch"])
	}
}

func TestWriteDoesNotArchiveConsumed(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	s := State{Branch: "main", Head: "abc1234", Health: 80}

	// Write and then archive manually (simulates a consumed doc)
	if err := Write(root, s, "graceful", now); err != nil {
		t.Fatal(err)
	}
	if _, err := Archive(root); err != nil {
		t.Fatal(err)
	}

	// At this point the active doc is gone. Write a new one.
	if err := Write(root, s, "urgent", now); err != nil {
		t.Fatalf("Write after archive: %v", err)
	}

	// Should have exactly 1 archived file (the manually archived one), not 2
	archivesDir := filepath.Join(root, ".claude", "project", "handoffs")
	entries, err := os.ReadDir(archivesDir)
	if err != nil {
		t.Fatalf("handoffs dir missing: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 archived file, got %d", len(entries))
	}
}
