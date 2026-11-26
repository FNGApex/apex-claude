package main

import (
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
	"apex/internal/repo"
	"apex/internal/skills"
)

// skillCmd validates every flag and argument before reading the disk, so usage errors always exit 64.
func skillCmd(dir string, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageErr("skill needs a verb: list, search, show or check")
	}
	sub, args := args[0], args[1:]
	if sub == "-h" || sub == "--help" {
		return flag.ErrHelp
	}
	fs := flag.NewFlagSet("skill "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "")
	kind, tag, source, limit, body := new(string), new(string), new(string), new(int), new(bool)
	switch sub {
	case "list":
		kind, tag, source = fs.String("kind", "", ""), fs.String("tag", "", ""), fs.String("source", "", "")
	case "search":
		kind, tag, limit = fs.String("kind", "", ""), fs.String("tag", "", ""), fs.Int("limit", 10, "")
	case "show":
		body = fs.Bool("body", false, "")
	case "check":
	default:
		return usageErr("unknown skill verb %q", sub)
	}
	pos, err := parseInterleaved(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if err != nil {
		return usageErr("%v", err)
	}
	switch {
	case sub == "list" && len(pos) != 0:
		return usageErr("skill list takes no arguments, got %d", len(pos))
	case sub == "search" && len(pos) == 0:
		return usageErr("skill search needs a query")
	case sub == "show" && len(pos) != 1:
		return usageErr("skill show takes 1 argument, got %d", len(pos))
	case sub == "check" && len(pos) > 1:
		return usageErr("skill check takes at most 1 argument, got %d", len(pos))
	case *kind != "" && !slices.Contains(skills.Kinds, *kind):
		return usageErr("unknown kind %q (one of %s)", *kind, strings.Join(skills.Kinds, ", "))
	case *source != "" && !slices.Contains(layout.Sources, *source):
		return usageErr("unknown source %q (one of %s)", *source, strings.Join(layout.Sources, ", "))
	case sub == "search" && *limit < 1:
		return usageErr("--limit must be positive, got %d", *limit)
	}

	root, err := repo.Main(dir)
	if err != nil {
		return err
	}
	warn := func(ws []string) {
		for _, w := range ws {
			fmt.Fprintf(stderr, "apex skill: warning: %s\n", w)
		}
	}
	if sub == "check" {
		return checkSkills(dir, root, pos, *asJSON, stdout, warn)
	}
	all, warnings := skills.Discover(layout.SkillRoots(root))
	warn(warnings)
	keep := func(s *skills.Skill) bool {
		return (*kind == "" || s.Kind == *kind) && (*source == "" || s.Source == *source) &&
			(*tag == "" || slices.ContainsFunc(s.Tags, func(t string) bool { return strings.EqualFold(t, *tag) }))
	}
	switch sub {
	case "list":
		out := []*skills.Skill{}
		for _, s := range all {
			if keep(s) {
				out = append(out, s)
			}
		}
		if *asJSON {
			return printJSON(stdout, out)
		}
		for _, s := range out {
			fmt.Fprintf(stdout, "%-24s  %-12s  %-11s  %s\n", s.Ref(), s.Kind, s.Source, s.Description)
		}
		fmt.Fprintf(stdout, "%d skills\n", len(out))
		return nil
	case "search":
		var pool []*skills.Skill
		for _, s := range all {
			if keep(s) {
				pool = append(pool, s)
			}
		}
		hits := skills.Search(pool, strings.Join(pos, " "), *limit)
		if *asJSON {
			if hits == nil {
				hits = []skills.Hit{}
			}
			return printJSON(stdout, hits)
		}
		for _, h := range hits {
			fmt.Fprintf(stdout, "%3d  %-24s  %-12s  %s\n", h.Score, h.Ref(), h.Kind, h.Description)
		}
		fmt.Fprintf(stdout, "%d matches\n", len(hits))
		return nil
	default: // show
		s := skills.Find(all, pos[0])
		if s == nil {
			return fmt.Errorf("%w: no skill %q", exit.NotFound, pos[0])
		}
		var text string
		if *body {
			data, err := os.ReadFile(s.Path)
			if err != nil {
				return err
			}
			text = string(data)
		}
		if *asJSON {
			v := struct {
				*skills.Skill
				Body string `json:"body,omitempty"`
			}{s, text}
			return printJSON(stdout, v)
		}
		for _, p := range [][2]string{
			{"name", s.Name}, {"description", s.Description}, {"kind", s.Kind}, {"tier", s.Tier},
			{"tags", strings.Join(s.Tags, ", ")}, {"triggers", strings.Join(s.Triggers, ", ")},
			{"inputs", strings.Join(s.Inputs, ", ")}, {"outputs", strings.Join(s.Outputs, ", ")},
			{"verify", strings.Join(s.Verify, ", ")}, {"requires", strings.Join(s.Requires, ", ")},
			{"path", s.Path}, {"source", s.Source}, {"domain", s.Domain}, {"shadows", strings.Join(s.Shadows, ", ")},
		} {
			if p[1] != "" {
				fmt.Fprintf(stdout, "%s: %s\n", p[0], p[1])
			}
		}
		if *body {
			fmt.Fprintf(stdout, "\n%s", text)
		}
		return nil
	}
}

// checkSkills validates the named path or skill, or by default the repo's own skill roots and $APEX_SKILLS_DIR.
func checkSkills(dir, root string, pos []string, asJSON bool, stdout io.Writer, warn func([]string)) error {
	var found []*skills.Skill
	if len(pos) == 0 {
		for i, r := range layout.SkillRoots(root) {
			if r.Source == "repo" || r.Source == "repo-claude" || r.Source == "env" {
				f, w := skills.Walk(r, i)
				warn(w)
				found = append(found, f...)
			}
		}
	} else {
		p := pos[0]
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		fi, err := os.Stat(p)
		switch {
		case err == nil:
			if !fi.IsDir() {
				if filepath.Base(p) != "SKILL.md" {
					return usageErr("%s is not a SKILL.md or a directory", pos[0])
				}
				p = filepath.Dir(p)
			}
			f, w := skills.Walk(layout.Root{Path: p, Source: "path"}, 0)
			warn(w)
			found = f
		case errors.Is(err, os.ErrNotExist):
			all, w := skills.Discover(layout.SkillRoots(root))
			warn(w)
			s := skills.Find(all, pos[0])
			if s == nil {
				return fmt.Errorf("%w: no skill or path %q", exit.NotFound, pos[0])
			}
			found = []*skills.Skill{s}
		default:
			return err
		}
	}
	type problem struct {
		Path    string `json:"path"`
		Problem string `json:"problem"`
	}
	problems := []problem{}
	for _, s := range found {
		for _, b := range skills.Check(s) {
			problems = append(problems, problem{s.Path, b})
		}
	}
	if asJSON {
		printJSON(stdout, map[string]any{"checked": len(found), "problems": problems})
	} else {
		for _, p := range problems {
			fmt.Fprintf(stdout, "%s: %s\n", p.Path, p.Problem)
		}
		fmt.Fprintf(stdout, "%d skills checked, %d problems\n", len(found), len(problems))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %d skill schema problem(s)", exit.Corrupt, len(problems))
	}
	return nil
}
