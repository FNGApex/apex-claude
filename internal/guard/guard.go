// Package guard implements the PreToolUse(Bash) safety hook: it blocks a narrow,
// high-confidence set of destructive commands. False blocks erode trust, so the
// bar is "unambiguously destructive", not "looks risky".
package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

var (
	rmRoot          = regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s+(/|~|\$HOME)(\s|$)`)
	pushForce       = regexp.MustCompile(`git\s+push\b.*--force([^-]|$|=)`) // --force-with-lease is NOT matched
	protectedBranch = regexp.MustCompile(`\b(main|master)\b`)
	curlPipeSh      = regexp.MustCompile(`(curl|wget)\s.*\|\s*(sudo\s+)?(ba)?sh\b`)
)

// Evaluate returns a deny reason and true when cmd should be blocked.
func Evaluate(cmd string) (reason string, deny bool) {
	switch {
	case rmRoot.MatchString(cmd):
		return "Refusing rm -rf on a root/home path.", true
	case pushForce.MatchString(cmd) && protectedBranch.MatchString(cmd):
		return "Refusing force-push to main/master. Use --force-with-lease on a feature branch.", true
	case curlPipeSh.MatchString(cmd):
		return "Refusing curl|sh — download, inspect, then run.", true
	}
	return "", false
}

// PreBash reads a PreToolUse hook payload from r, and on a destructive command
// writes a deny payload to w and returns exit code 2. Otherwise returns 0.
func PreBash(r io.Reader, w io.Writer) int {
	data, _ := io.ReadAll(r)

	var payload struct {
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	cmd := string(data) // degrade to raw scan if the payload isn't the expected shape
	if err := json.Unmarshal(data, &payload); err == nil && payload.ToolInput.Command != "" {
		cmd = payload.ToolInput.Command
	}

	reason, deny := Evaluate(cmd)
	if !deny {
		return 0
	}
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	}
	b, _ := json.Marshal(out)
	fmt.Fprintln(w, string(b))
	return 2
}
