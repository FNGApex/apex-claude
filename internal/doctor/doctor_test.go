package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeArtifacts lays down the shared artifact surface both layouts require:
// one output-style, agent, command, and a skill dir with a SKILL.md.
func writeArtifacts(t *testing.T, root string) {
	t.Helper()
	must := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(root, "output-styles", "apex.md"), "# style")
	must(filepath.Join(root, "agents", "ax-x.md"), "# agent")
	must(filepath.Join(root, "commands", "ax-x.md"), "# command")
	must(filepath.Join(root, "skills", "ax-x", "SKILL.md"), "# skill")
}

func run(t *testing.T, root string) (int, string) {
	t.Helper()
	t.Setenv("CLAUDE_PLUGIN_ROOT", root)
	var buf bytes.Buffer
	code := Run(&buf)
	return code, buf.String()
}

func TestLooseInstallPasses(t *testing.T) {
	root := t.TempDir()
	writeArtifacts(t, root)
	os.WriteFile(filepath.Join(root, "settings.json"),
		[]byte(`{"hooks":{"PreToolUse":[{"hooks":[{"command":"/x/apex hooks pre-bash"}]}]}}`), 0o644)

	code, out := run(t, root)
	if code != 0 {
		t.Fatalf("want pass, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "loose install") {
		t.Errorf("expected loose-install layout label\n%s", out)
	}
	// Loose layout must NOT enforce the plugin-era contract.
	if strings.Contains(out, "plugin.json") {
		t.Errorf("loose install should not check plugin.json\n%s", out)
	}
	if !strings.Contains(out, "apex hooks wired in settings.json") {
		t.Errorf("expected the settings.json hook-wiring check\n%s", out)
	}
}

func TestLooseInstallFailsWithoutWiredHooks(t *testing.T) {
	root := t.TempDir()
	writeArtifacts(t, root)
	os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"hooks":{}}`), 0o644)

	code, out := run(t, root)
	if code != 1 {
		t.Fatalf("want failure when hooks unwired, got %d\n%s", code, out)
	}
}

func TestLooseInstallFailsWithoutSettingsFile(t *testing.T) {
	root := t.TempDir()
	writeArtifacts(t, root)
	// No settings.json at all — apexHooksWired must treat the read error as unwired.

	code, _ := run(t, root)
	if code != 1 {
		t.Fatalf("want failure when settings.json is absent, got %d", code)
	}
}

func TestDevLayoutChecksManifest(t *testing.T) {
	root := t.TempDir()
	writeArtifacts(t, root)
	os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755)
	os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(`{"name":"x"}`), 0o644)
	os.MkdirAll(filepath.Join(root, "hooks"), 0o755)
	os.WriteFile(filepath.Join(root, "hooks", "hooks.json"), []byte(`{}`), 0o644)

	code, out := run(t, root)
	if code != 0 {
		t.Fatalf("want pass, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "dev/plugin layout") {
		t.Errorf("expected dev/plugin layout label\n%s", out)
	}
	if !strings.Contains(out, "plugin.json is valid JSON") {
		t.Errorf("dev layout should check plugin.json\n%s", out)
	}
}

func TestDevLayoutFailsOnBadManifest(t *testing.T) {
	root := t.TempDir()
	writeArtifacts(t, root)
	os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755)
	os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(`{bad json`), 0o644)
	// Valid hooks.json so the bad manifest is the *only* failing check.
	os.MkdirAll(filepath.Join(root, "hooks"), 0o755)
	os.WriteFile(filepath.Join(root, "hooks", "hooks.json"), []byte(`{}`), 0o644)

	code, _ := run(t, root)
	if code != 1 {
		t.Fatalf("want failure on invalid plugin.json, got %d", code)
	}
}
