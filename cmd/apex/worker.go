package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"apex/internal/repo"
	"apex/internal/worker"
)

// workerView is a manifest with its derived worktree path and, unless removed, what git says now.
type workerView struct {
	worker.Manifest
	Worktree string       `json:"worktree"`
	Live     *worker.Live `json:"live,omitempty"`
}

// workerCmd validates every flag and argument before touching git or the disk, so usage errors always exit 64.
func workerCmd(dir string, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageErr("worker needs a verb: spawn, adopt, list, status, merge, remove or prune")
	}
	sub, args := args[0], args[1:]
	if sub == "-h" || sub == "--help" {
		return flag.ErrHelp
	}
	fs := flag.NewFlagSet("worker "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "")
	var base, skill, item, states, message, slug *string
	var force, purge *bool
	want := 1
	switch sub {
	case "spawn":
		base, skill, item = fs.String("base", "", ""), fs.String("skill", "", ""), fs.String("item", "", "")
	case "adopt":
		base, skill, item = fs.String("base", "", ""), fs.String("skill", "", ""), fs.String("item", "", "")
		slug = fs.String("slug", "", "")
	case "list":
		states, want = fs.String("state", "active,merged", ""), 0
	case "status":
	case "merge":
		message = fs.String("message", "", "")
	case "remove":
		force, purge = fs.Bool("force", false, ""), fs.Bool("purge", false, "")
	case "prune":
		want = 0
	default:
		return usageErr("unknown worker verb %q", sub)
	}
	pos, err := parseInterleaved(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if err != nil {
		return usageErr("%v", err)
	}
	if len(pos) != want {
		return usageErr("worker %s takes %d argument(s), got %d", sub, want, len(pos))
	}
	var id int
	var skills []string
	var show []string
	switch sub {
	case "spawn":
		err = worker.CheckSlug(pos[0])
		if err == nil {
			skills, err = worker.ParseSkills(*skill)
		}
		if err == nil {
			err = worker.CheckItem(*item)
		}
	case "adopt":
		if pos[0] == "" || strings.ContainsFunc(pos[0], unicode.IsControl) {
			return usageErr("adopt needs a worktree path or branch, got %q", pos[0])
		}
		if *slug != "" {
			err = worker.CheckSlug(*slug)
		}
		if err == nil {
			skills, err = worker.ParseSkills(*skill)
		}
		if err == nil {
			err = worker.CheckItem(*item)
		}
	case "list":
		show, err = parseWorkerStates(*states)
	case "status", "merge", "remove":
		id, err = worker.ParseID(pos[0])
	}
	if err != nil {
		return err
	}

	root, err := repo.Main(dir)
	if err != nil {
		return err
	}
	m := worker.New(root)
	view := func(mf worker.Manifest) (workerView, error) {
		v := workerView{Manifest: mf, Worktree: m.WorktreePath(mf)}
		if mf.State != worker.StateRemoved {
			lv, err := m.Live(mf)
			if err != nil {
				return v, err
			}
			v.Live = &lv
		}
		return v, nil
	}
	switch sub {
	case "spawn":
		mf, err := m.Spawn(dir, pos[0], *base, skills, *item)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(stdout, workerView{Manifest: mf, Worktree: m.WorktreePath(mf)})
		}
		fmt.Fprintf(stdout, "id: %s\nworktree: %s\nbranch: %s\nbase: %s\n", mf.ID, m.Worktree(mf.ID), mf.Branch, mf.Base)
		return nil
	case "adopt":
		mf, err := m.Adopt(dir, pos[0], *slug, *base, skills, *item)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(stdout, workerView{Manifest: mf, Worktree: m.WorktreePath(mf)})
		}
		wt := m.WorktreePath(mf)
		if wt == "" {
			wt = "(branch only)"
		}
		fmt.Fprintf(stdout, "id: %s\nworktree: %s\nbranch: %s\nbase: %s\n", mf.ID, wt, mf.Branch, mf.Base)
		return nil
	case "list":
		all, err := m.List()
		if err != nil {
			return err
		}
		views := []workerView{}
		counts := map[string]int{}
		for _, mf := range all {
			counts[mf.State]++
			if !slices.Contains(show, mf.State) {
				continue
			}
			v, err := view(mf)
			if err != nil {
				return err
			}
			views = append(views, v)
		}
		if *asJSON {
			return printJSON(stdout, views)
		}
		for _, v := range views {
			fmt.Fprintln(stdout, workerRow(v))
		}
		fmt.Fprintf(stdout, "%d active, %d merged, %d removed\n",
			counts[worker.StateActive], counts[worker.StateMerged], counts[worker.StateRemoved])
		return nil
	case "status":
		mf, err := m.Get(id)
		if err != nil {
			return err
		}
		v, err := view(mf)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(stdout, v)
		}
		printWorker(stdout, v)
		return nil
	case "merge":
		mf, err := m.Merge(id, *message)
		var ce *worker.ConflictError
		if errors.As(err, &ce) && *asJSON {
			printJSON(stdout, map[string]any{"id": ce.ID, "conflict": true, "files": ce.Files})
		}
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(stdout, workerView{Manifest: mf, Worktree: m.WorktreePath(mf)})
		}
		fmt.Fprintf(stdout, "merged %s into %s at %s\n", mf.ID, mf.Base, mf.MergedSHA)
		return nil
	case "remove":
		mf, err := m.Remove(id, *force, *purge)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(stdout, workerView{Manifest: mf, Worktree: m.WorktreePath(mf)})
		}
		fmt.Fprintf(stdout, "removed %s\n", mf.ID)
		return nil
	default: // prune
		removed, skipped, err := m.Prune()
		if *asJSON && err == nil {
			ids := func(ms []worker.Manifest) []string {
				out := []string{}
				for _, mf := range ms {
					out = append(out, mf.ID)
				}
				return out
			}
			return printJSON(stdout, map[string][]string{"removed": ids(removed), "skipped": ids(skipped)})
		}
		for _, mf := range removed {
			fmt.Fprintf(stdout, "removed %s\n", mf.ID)
		}
		for _, mf := range skipped {
			fmt.Fprintf(stdout, "skipped %s (uncommitted or unmerged work)\n", mf.ID)
		}
		if err == nil {
			fmt.Fprintf(stdout, "%d removed, %d skipped\n", len(removed), len(skipped))
		}
		return err
	}
}

func parseWorkerStates(s string) ([]string, error) {
	var show []string
	for _, st := range strings.Split(s, ",") {
		switch st = strings.TrimSpace(st); {
		case st == "all":
			show = append(show, worker.States...)
		case slices.Contains(worker.States, st):
			show = append(show, st)
		default:
			return nil, usageErr("unknown state %q", st)
		}
	}
	return show, nil
}

func workerRow(v workerView) string {
	live := "-"
	if v.Live != nil {
		live = fmt.Sprintf("%-7s  ahead %d  behind %d  dirty %d", v.Live.Worktree, v.Live.Ahead, v.Live.Behind, v.Live.Dirty)
	}
	return fmt.Sprintf("%s  %-7s  %-40s  %s", v.ID, v.State, v.Branch+" <- "+v.Base, live)
}

func printWorker(w io.Writer, v workerView) {
	kv := [][2]string{
		{"id", v.ID}, {"slug", v.Slug}, {"state", v.State}, {"branch", v.Branch}, {"base", v.Base},
		{"base_sha", v.BaseSHA}, {"worktree", v.Worktree}, {"skills", strings.Join(v.Skills, ",")}, {"item", v.Item},
		{"created", v.Created}, {"merged", v.Merged}, {"merged_sha", v.MergedSHA}, {"removed", v.Removed},
	}
	if v.Live != nil {
		kv = append(kv, [2]string{"live", v.Live.Worktree}, [2]string{"head", v.Live.Head},
			[2]string{"dirty", strconv.Itoa(v.Live.Dirty)}, [2]string{"ahead", strconv.Itoa(v.Live.Ahead)},
			[2]string{"behind", strconv.Itoa(v.Live.Behind)})
	}
	for _, p := range kv {
		if p[1] != "" {
			fmt.Fprintf(w, "%s: %s\n", p[0], p[1])
		}
	}
}
