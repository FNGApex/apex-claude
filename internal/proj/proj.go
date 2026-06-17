// Package proj resolves project-scoped paths for the apex backbone.
package proj

import (
	"os"
	"path/filepath"
)

// Root resolves the project root: $APEX_REPO if set, else the working directory.
func Root() string {
	if r := os.Getenv("APEX_REPO"); r != "" {
		return r
	}
	wd, _ := os.Getwd()
	return wd
}

// StateDir returns <root>/.claude/project, creating it if needed.
func StateDir(root string) (string, error) {
	d := filepath.Join(root, ".claude", "project")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	return d, nil
}
