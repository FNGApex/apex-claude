// Command apex is the deterministic backbone of the Agent protocol: it resolves the main checkout and its layout, keeps
// each Agent's ledger, manages an Orchestrator's worker worktrees, and indexes skills.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"apex/internal/exit"
	"apex/internal/layout"
	"apex/internal/ledger"
	"apex/internal/repo"
)

const usage = `usage: apex <verb> [args] --agent <name>

  add <task|followup> <title>          file a new open item, print its path
  list [--status s,s|all] [--kind k]   default status: in-progress,open
  start <id>                           open -> in-progress
  close <id> <reason>                  -> closed
  reopen <id>                          in-progress|closed -> open
  retitle <id> <title>                 replace an item's title, any status

an Agent whose ledger dir holds a file named per-ticket keeps one ledger per ticket: the ticket id in the current
branch (ENG-42), else the branch name; detached or outside git, the mono ledger. One holding a file named concurrent
may have several items in progress.

usage: apex where [--agent <name>] [--json]
  print the resolved root, layout (apex: .apex/ + .agents/skills/, legacy: .claude/) and every protocol path

usage: apex worker <verb> [args] [--json]
  spawn <slug> [--base b] [--skill a,b] [--item ref]   new worktree on branch apex/<id>-<slug> under .apex/workers/
  list [--state s,s|all]                                default state: active,merged; shows live git state
  status <id>                                           one worker
  merge <id> [--message m]                              merge a clean worker into its base, where base is checked out
  remove <id> [--force]                                 drop worktree and branch; --force drops unmerged work
  prune                                                 remove every merged worker with nothing unmerged

usage: apex skill <verb> [args] [--json]
  list [--kind k] [--tag t] [--source s]                every discovered skill, first of a name wins
  search <query...> [--kind k] [--tag t] [--limit n]    best matches, default limit 10
  show <name|domain/name> [--body]                      one skill's fields, and with --body its file
  check [<path|name>]                                   validate frontmatter; default: the repo's skills

exit: 0 ok, 1 not found, 2 environment, 3 illegal transition, 4 corrupt state, 5 merge conflict, 64 usage
`

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "apex:", err)
		os.Exit(2)
	}
	os.Exit(run(dir, os.Args[1:], os.Stdout, os.Stderr))
}

func run(dir string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 64
	}
	verb, args := args[0], args[1:]
	if verb == "-h" || verb == "--help" || verb == "help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	err := dispatch(dir, verb, args, stdout, stderr)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "apex %s: %v\n", verb, err)
	}
	return exit.Code(err)
}

func usageErr(format string, a ...any) error {
	return fmt.Errorf("%w: %s", exit.Usage, fmt.Sprintf(format, a...))
}

func printJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// dispatch validates every argument before reading the disk, so a usage error exits 64 whatever state the ledger is in.
func dispatch(dir, verb string, args []string, stdout, stderr io.Writer) error {
	switch verb {
	case "where":
		return where(dir, args, stdout)
	case "worker":
		return workerCmd(dir, args, stdout)
	case "skill":
		return skillCmd(dir, args, stdout, stderr)
	}
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	agent := fs.String("agent", "", "")
	var status, kind *string
	var want int
	switch verb {
	case "add":
		want = 2
	case "list":
		status = fs.String("status", "in-progress,open", "")
		kind = fs.String("kind", "", "")
	case "start", "reopen":
		want = 1
	case "close", "retitle":
		want = 2
	default:
		return usageErr("unknown verb %q", verb)
	}
	pos, err := parseInterleaved(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if err != nil {
		return usageErr("%v", err)
	}
	if len(pos) != want {
		return usageErr("%s takes %d argument(s), got %d", verb, want, len(pos))
	}
	if *agent == "" {
		return usageErr("--agent is required")
	}
	if err := ledger.CheckAgent(*agent); err != nil {
		return err
	}

	var id int
	var show map[string]bool
	switch verb {
	case "add":
		err = ledger.CheckNew(pos[0], pos[1])
	case "list":
		show, err = parseStatuses(*status)
		if err == nil && *kind != "" && !ledger.ValidKind(*kind) {
			err = usageErr("unknown kind %q", *kind)
		}
	default:
		id, err = ledger.ParseID(pos[0])
		if err == nil && verb == "close" {
			err = ledger.CheckReason(pos[1])
		}
		if err == nil && verb == "retitle" {
			err = ledger.CheckTitle(pos[1])
		}
	}
	if err != nil {
		return err
	}

	root, err := repo.Main(dir)
	if err != nil {
		return err
	}
	l, ticketed, err := openLedger(dir, root, *agent)
	if err != nil {
		return err
	}
	switch verb {
	case "add":
		_, path, err := l.Add(pos[0], pos[1])
		if err == nil {
			fmt.Fprintln(stdout, path)
		}
		return err
	case "list":
		if ticketed {
			fmt.Fprintf(stdout, "ledger: %s\n", l.Dir())
		}
		return list(l, show, *kind, stdout)
	case "start":
		err = l.Start(id)
	case "reopen":
		err = l.Reopen(id)
	case "close":
		err = l.Close(id, pos[1])
	case "retitle":
		err = l.Retitle(id, pos[1])
	}
	if err == nil {
		fmt.Fprintln(stdout, l.Path(id))
	}
	return err
}

// openLedger opens the Agent's ledger, switching to the current ticket's ledger when the Agent keeps one per ticket.
func openLedger(dir, root, agent string) (*ledger.Ledger, bool, error) {
	l, err := ledger.Open(root, agent)
	if err != nil || !l.PerTicket() {
		return l, false, err
	}
	// Off a branch (detached, outside git) the Agent falls back to its mono ledger.
	ticket, err := repo.Ticket(dir)
	if err != nil || ticket == "" {
		return l, false, err
	}
	l, err = l.ForTicket(ticket)
	return l, err == nil, err
}

// where prints every path the protocol resolves, for the repo and optionally one Agent, so skills never hard-code a
// layout.
func where(dir string, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("where", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	agent := fs.String("agent", "", "")
	asJSON := fs.Bool("json", false, "")
	pos, err := parseInterleaved(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if err != nil {
		return usageErr("%v", err)
	}
	if len(pos) != 0 {
		return usageErr("where takes no arguments, got %d", len(pos))
	}
	if *agent != "" {
		if err := ledger.CheckAgent(*agent); err != nil {
			return err
		}
	}
	root, err := repo.Main(dir)
	if err != nil {
		return err
	}
	lay := layout.ForRepo(root)
	if *agent != "" {
		if lay, err = layout.ForAgent(root, *agent); err != nil {
			return err
		}
	}
	type kv struct {
		k string
		v any
	}
	out := []kv{
		{"root", root}, {"layout", lay.Name}, {"state", filepath.Join(root, lay.State)},
		{"skills", filepath.Join(root, lay.Skills)}, {"framing", lay.FramingFile(root)},
		{"workers", layout.Workers(root)}, {"live", lay.Shared(root, "LIVE.md")}, {"git", lay.Shared(root, "git.md")},
		{"tiers", lay.Shared(root, "tiers.md")},
	}
	if *agent != "" {
		l, _, err := openLedger(dir, root, *agent)
		if err != nil {
			return err
		}
		mono, err := ledger.Open(root, *agent)
		if err != nil {
			return err
		}
		out = append(out, kv{"skill", lay.Skill(root, *agent)}, kv{"ledger", l.Dir()},
			kv{"handoff", filepath.Join(l.Dir(), "handoff.md")}, kv{"private", lay.Private(root, *agent)},
			kv{"per_ticket", mono.PerTicket()}, kv{"concurrent", l.Concurrent()})
	}
	if *asJSON {
		m := map[string]any{}
		for _, p := range out {
			m[p.k] = p.v
		}
		return printJSON(stdout, m)
	}
	for _, p := range out {
		fmt.Fprintf(stdout, "%s: %v\n", p.k, p.v)
	}
	return nil
}

// parseInterleaved lets flags appear before, between, or after positionals. Everything after "--" is positional.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var tail []string
	if i := slices.Index(args, "--"); i >= 0 {
		args, tail = args[:i], args[i+1:]
	}
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return append(pos, tail...), nil
		}
		pos, args = append(pos, args[0]), args[1:]
	}
}

func parseStatuses(s string) (map[string]bool, error) {
	show := map[string]bool{}
	for _, st := range strings.Split(s, ",") {
		switch st = strings.TrimSpace(st); {
		case st == "all":
			show[ledger.StatusOpen], show[ledger.StatusInProgress], show[ledger.StatusClosed] = true, true, true
		case ledger.ValidStatus(st):
			show[st] = true
		default:
			return nil, usageErr("unknown status %q", st)
		}
	}
	return show, nil
}

func list(l *ledger.Ledger, show map[string]bool, kind string, stdout io.Writer) error {
	items, err := l.List()
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, it := range items {
		if kind != "" && it.Kind != kind {
			continue
		}
		counts[it.Status]++
		if show[it.Status] {
			fmt.Fprintf(stdout, "%03d  %-11s  %-8s  %s\n", it.ID, it.Status, it.Kind, it.Title)
		}
	}
	fmt.Fprintf(stdout, "%d open, %d in-progress, %d closed\n",
		counts[ledger.StatusOpen], counts[ledger.StatusInProgress], counts[ledger.StatusClosed])
	return nil
}
