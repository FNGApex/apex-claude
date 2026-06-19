package main

import (
	"fmt"
	"os"
	"time"

	"apexclaude/internal/handoff"
	"apexclaude/internal/proj"
)

func init() {
	register("handoff", "scan/status/archive handoff documents", runHandoff)
}

func runHandoff(args []string) int {
	root := proj.Root()
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "scan":
		mode := "graceful"
		if len(args) > 1 {
			mode = args[1]
		}
		if mode != "graceful" && mode != "urgent" {
			fmt.Fprintln(os.Stderr, "usage: apex handoff scan [graceful|urgent]")
			return 2
		}
		s, err := handoff.Scan(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "handoff scan:", err)
			return 1
		}
		if err := handoff.Write(root, s, mode, time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, "handoff scan:", err)
			return 1
		}
		fmt.Println(handoff.Path(root))
		return 0

	case "status":
		code := handoff.Status(root)
		switch code {
		case 0:
			fmt.Println("handoff present, fresh")
		case 2:
			fmt.Println("handoff present, STALE (HEAD moved)")
		default:
			fmt.Println("no active handoff")
		}
		return code

	case "archive":
		id, err := handoff.Archive(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "handoff archive:", err)
			return 1
		}
		fmt.Printf("archived to .claude/project/handoffs/%s.md\n", id)
		return 0

	default:
		fmt.Fprintln(os.Stderr, "usage: apex handoff <scan|status|archive>")
		return 2
	}
}
