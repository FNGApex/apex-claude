package layout

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLooksLikeArtifactRoot(t *testing.T) {
	empty := t.TempDir()
	if LooksLikeArtifactRoot(empty) {
		t.Error("empty dir must not look like an artifact root")
	}
	loose := t.TempDir()
	os.MkdirAll(filepath.Join(loose, "commands"), 0o755)
	if !LooksLikeArtifactRoot(loose) {
		t.Error("dir with commands/ should look like an artifact root (loose layout)")
	}
	dev := t.TempDir()
	os.MkdirAll(filepath.Join(dev, ".claude-plugin"), 0o755)
	if !LooksLikeArtifactRoot(dev) {
		t.Error("dir with .claude-plugin/ should look like an artifact root (dev layout)")
	}
}
