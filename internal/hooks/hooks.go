// Package hooks implements the SessionStart hook payload: it surfaces project
// nudges (stale signals, and later due reminders) as additionalContext. Emits
// nothing when there is nothing pending.
package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"apexclaude/internal/proj"
	"apexclaude/internal/reminder"
	"apexclaude/internal/signals"
)

// SessionStart writes a SessionStart hook payload to w. Returns 0 always (a hook
// failure should never block a session). No output when nothing is pending.
func SessionStart(args []string, w io.Writer) int {
	root := proj.Root()

	var nudges []string
	if code, reason := signals.Stale(root); code == 1 {
		nudges = append(nudges, "Project signals stale: "+reason)
	}
	if due, _ := reminder.Due(root, time.Now()); len(due) > 0 {
		for _, r := range due {
			nudges = append(nudges, "Reminder due: "+r.Text)
		}
	}

	if len(nudges) == 0 {
		return 0
	}

	ctx := "Apex session start:\n- " + strings.Join(nudges, "\n- ")
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SessionStart",
			"additionalContext": ctx,
		},
	}
	b, _ := json.Marshal(out)
	fmt.Fprintln(w, string(b))
	return 0
}
