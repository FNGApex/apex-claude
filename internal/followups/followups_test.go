package followups

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddListRenderClose(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)

	fid, err := Add(root, "drop the bash guard", "finding", "risk", "review", "body", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Add(root, "wiki subsystem", "plan", "", "design", "deferred", now); err != nil {
		t.Fatal(err)
	}

	list, _ := List(root)
	if len(list) != 2 {
		t.Fatalf("List len=%d, want 2", len(list))
	}

	idx, err := os.ReadFile(filepath.Join(root, ".claude", "project", "followups", "INDEX.md"))
	if err != nil {
		t.Fatalf("INDEX.md missing: %v", err)
	}
	body := string(idx)
	if !strings.Contains(body, "📋 plans") || !strings.Contains(body, "wiki subsystem") {
		t.Errorf("INDEX missing plans section:\n%s", body)
	}
	if !strings.Contains(body, "drop the bash guard") {
		t.Errorf("INDEX missing finding:\n%s", body)
	}

	if err := Close(root, fid, "done"); err != nil {
		t.Fatal(err)
	}
	if list, _ := List(root); len(list) != 1 {
		t.Fatalf("after close len=%d, want 1", len(list))
	}
	closed, err := os.ReadFile(filepath.Join(root, ".claude", "project", "followups", "CLOSED.md"))
	if err != nil || !strings.Contains(string(closed), "drop the bash guard") {
		t.Errorf("CLOSED.md missing entry: %v\n%s", err, closed)
	}
}
