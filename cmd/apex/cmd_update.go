package main

import (
	"fmt"
	"os"

	"apexclaude/internal/update"
	"apexclaude/internal/version"
)

func init() {
	register("update", "check for Apex Claude releases", runUpdate)
}

func runUpdate(args []string) int {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "check":
		return runUpdateCheck(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: apex update <check>")
		return 2
	}
}

// runUpdateCheck refreshes the update cache (a network fetch, regardless of
// TTL — TTL gating is the session-start hook's job) and reports whether a
// newer release is available. --quiet keeps the exit code but suppresses the
// stdout report, matching the form a hook spawns detached; failure
// diagnostics still go to stderr.
func runUpdateCheck(args []string) int {
	quiet := false
	for _, a := range args {
		if a == "--quiet" {
			quiet = true
		}
	}

	cur := "v" + version.Version
	c, err := update.Refresh(update.CachePath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "apex update check:", err)
		return 2
	}
	if update.Compare(c.Latest, cur) > 0 {
		if !quiet {
			fmt.Printf("apex %s — latest %s (update available)\n", cur, c.Latest)
		}
		return 1
	}
	if !quiet {
		fmt.Printf("apex %s — up to date\n", cur)
	}
	return 0
}
