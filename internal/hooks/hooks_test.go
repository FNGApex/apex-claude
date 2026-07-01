package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A missing signals file is code 1 (stale) — the nudge must surface it.
func TestSessionStartSurfacesStaleSignals(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APEX_REPO", root)

	var buf bytes.Buffer
	if code := SessionStart(nil, &buf); code != 0 {
		t.Fatalf("hook must never fail the session, got %d", code)
	}
	if !strings.Contains(buf.String(), "Project signals stale") {
		t.Errorf("expected stale-signals nudge\n%s", buf.String())
	}
}

// A signals file that can't be read is code 2 (error) — that must be surfaced
// too, not swallowed. A directory at the file's path forces the read error.
func TestSessionStartSurfacesSignalsError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APEX_REPO", root)
	if err := os.MkdirAll(filepath.Join(root, ".claude", "project", "deterministic-signals.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if code := SessionStart(nil, &buf); code != 0 {
		t.Fatalf("hook must never fail the session, got %d", code)
	}
	if !strings.Contains(buf.String(), "Project signals check failed") {
		t.Errorf("expected signals-error nudge\n%s", buf.String())
	}
}
