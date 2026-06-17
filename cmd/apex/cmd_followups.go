package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"apexclaude/internal/followups"
	"apexclaude/internal/proj"
)

func init() {
	register("followups", "list/add/close/render/path follow-up records", runFollowups)
}

func runFollowups(args []string) int {
	root := proj.Root()
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "add":
		// apex followups add "<title>" [kind] [severity] [origin]
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, `usage: apex followups add "<title>" [finding|plan] [severity] [origin]`)
			return 2
		}
		kind, sev, origin := "", "", "cli"
		if len(args) > 2 {
			kind = args[2]
		}
		if len(args) > 3 {
			sev = args[3]
		}
		if len(args) > 4 {
			origin = strings.Join(args[4:], " ")
		}
		id, err := followups.Add(root, args[1], kind, sev, origin, "", time.Now())
		if err != nil {
			fmt.Fprintln(os.Stderr, "followups add:", err)
			return 2
		}
		fmt.Println("added follow-up", id)
		return 0
	case "list":
		list, err := followups.List(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "followups list:", err)
			return 2
		}
		if len(list) == 0 {
			fmt.Println("no open follow-ups")
			return 0
		}
		for _, e := range list {
			fmt.Printf("%s  %-7s  %-8s  %s\n", e.ID, e.Kind, e.Severity, e.Title)
		}
		return 0
	case "close":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: apex followups close <id> [reason]")
			return 2
		}
		if err := followups.Close(root, args[1], strings.Join(args[2:], " ")); err != nil {
			fmt.Fprintln(os.Stderr, "followups close:", err)
			return 1
		}
		fmt.Println("closed", args[1])
		return 0
	case "render":
		if err := followups.Render(root); err != nil {
			fmt.Fprintln(os.Stderr, "followups render:", err)
			return 2
		}
		fmt.Println("rendered INDEX.md")
		return 0
	case "path":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: apex followups path <id>")
			return 2
		}
		fmt.Println(followups.Path(root, args[1]))
		return 0
	default:
		fmt.Fprintln(os.Stderr, "usage: apex followups <list|add|close|render|path>")
		return 2
	}
}
