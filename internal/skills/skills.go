// Package skills indexes the SKILL.md files under every skill root, searches them, and checks them against the
// frontmatter schema the protocol relies on.
package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"apex/internal/layout"
)

var (
	Kinds = []string{"orchestrator", "worker-skill", "agent", "protocol", "skill"}
	Tiers = []string{"fast", "standard", "deep", "audit", "security"}

	nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

const maxDepth = 6

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"`
	Tags        []string `json:"tags"`
	Triggers    []string `json:"triggers"`
	Tier        string   `json:"tier,omitempty"`
	Inputs      []string `json:"inputs"`
	Outputs     []string `json:"outputs"`
	Verify      []string `json:"verify"`
	Requires    []string `json:"requires"`
	Path        string   `json:"path"`
	Source      string   `json:"source"`
	Domain      string   `json:"domain"`
	Shadows     []string `json:"shadows"`

	rank int
	dir  string
	fm   frontmatter
}

// Ref is how show and check name a skill: its name, or domain/name.
func (s *Skill) Ref() string {
	if s.Domain == "" {
		return s.Name
	}
	return s.Domain + "/" + s.Name
}

// Walk finds every SKILL.md under root, up to six levels deep, without descending into a skill, a dot-directory or
// node_modules. A missing root has none; unreadable directories become warnings.
func Walk(root layout.Root, rank int) (found []*Skill, warnings []string) {
	if _, err := os.Stat(root.Path); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			warnings = append(warnings, err.Error())
		}
		return nil, warnings
	}
	index := func(dir string) bool {
		s, err := load(root, rank, dir)
		if errors.Is(err, fs.ErrNotExist) {
			return false
		}
		if err != nil {
			warnings = append(warnings, err.Error())
		} else {
			found = append(found, s)
		}
		return true
	}
	filepath.WalkDir(root.Path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, err.Error())
			if d != nil && d.IsDir() && p != root.Path {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root.Path, p)
		if p != root.Path {
			if name := d.Name(); strings.HasPrefix(name, ".") || name == "node_modules" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if d.IsDir() {
			if rel != "." && strings.Count(rel, string(filepath.Separator))+1 > maxDepth {
				return filepath.SkipDir
			}
			if index(p) {
				return filepath.SkipDir
			}
			return nil
		}
		// A symlink or junction to a directory is indexed if it is a skill, never followed, so no loop is possible.
		if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				index(p)
			}
		}
		return nil
	})
	return found, warnings
}

func load(root layout.Root, rank int, dir string) (*Skill, error) {
	path := filepath.Join(dir, "SKILL.md")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fm := parseFrontmatter(f)
	s := &Skill{Path: path, Source: root.Source, rank: rank, dir: dir, fm: fm, Shadows: []string{}}
	if rel, err := filepath.Rel(root.Path, filepath.Dir(dir)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		s.Domain = filepath.ToSlash(rel)
	}
	s.Name = scalar(fm, "name")
	if s.Name == "" {
		s.Name = filepath.Base(dir)
	}
	s.Description = scalar(fm, "description")
	if s.Description == "" {
		s.Description = firstLine(fm.body)
	}
	s.Kind = strings.ToLower(scalar(fm, "kind"))
	if s.Kind == "" {
		s.Kind = "skill"
	}
	s.Tier = scalar(fm, "tier")
	s.Tags = list(fm, "tags", " ,")
	if len(s.Tags) == 0 && s.Domain != "" {
		s.Tags = strings.Split(s.Domain, "/")
	}
	s.Triggers = list(fm, "triggers", ",")
	s.Inputs = list(fm, "inputs", ",")
	s.Outputs = list(fm, "outputs", ",")
	s.Verify = list(fm, "verify", ",")
	s.Requires = list(fm, "requires", ",")
	return s, nil
}

func scalar(fm frontmatter, key string) string {
	v, _ := fm.get(key)
	if v.isList {
		return strings.Join(v.list, ", ")
	}
	return strings.TrimSpace(v.s)
}

// list reads a list field; a scalar is split on any of seps.
func list(fm frontmatter, key, seps string) []string {
	v, _ := fm.get(key)
	if v.isList {
		return v.list
	}
	out := []string{}
	for _, p := range strings.FieldsFunc(v.s, func(r rune) bool { return strings.ContainsRune(seps, r) }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstLine(body string) string {
	for _, l := range strings.Split(body, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			if r := []rune(l); len(r) > 200 {
				l = string(r[:200])
			}
			return l
		}
	}
	return ""
}

// Discover indexes every root in precedence order. The first skill of a name wins; later ones are its Shadows.
func Discover(roots []layout.Root) (skills []*Skill, warnings []string) {
	byName := map[string]*Skill{}
	for i, r := range roots {
		found, w := Walk(r, i)
		warnings = append(warnings, w...)
		for _, s := range found {
			if first, ok := byName[s.Name]; ok {
				first.Shadows = append(first.Shadows, s.Path)
				continue
			}
			byName[s.Name] = s
			skills = append(skills, s)
		}
	}
	sort.SliceStable(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, warnings
}

// Find returns the skill whose name or domain/name is ref.
func Find(skills []*Skill, ref string) *Skill {
	for _, s := range skills {
		if s.Name == ref || s.Ref() == ref {
			return s
		}
	}
	return nil
}

var tokenRe = regexp.MustCompile(`[a-z0-9]+`)

type Hit struct {
	*Skill
	Score int `json:"score"`
}

// Search scores skills by the query's lowercase alphanumeric tokens and returns the best limit hits.
func Search(skills []*Skill, query string, limit int) []Hit {
	tokens := tokenRe.FindAllString(strings.ToLower(query), -1)
	var hits []Hit
	for _, s := range skills {
		name, desc, domain := strings.ToLower(s.Name), strings.ToLower(s.Description), strings.ToLower(s.Domain)
		score := 0
		for _, t := range tokens {
			switch {
			case name == t:
				score += 8
			case strings.Contains(name, t):
				score += 6
			}
			if slices.ContainsFunc(s.Tags, func(g string) bool { return strings.ToLower(g) == t }) {
				score += 5
			}
			if slices.ContainsFunc(s.Triggers, func(g string) bool { return strings.Contains(strings.ToLower(g), t) }) {
				score += 4
			}
			if strings.Contains(desc, t) || strings.Contains(domain, t) {
				score += 2
			}
		}
		if score > 0 {
			hits = append(hits, Hit{s, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		return a.Name < b.Name
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// Check lists every way s breaks the schema: frontmatter with an explicit name that matches its directory, an explicit
// one-line description, and known kind and tier values.
func Check(s *Skill) []string {
	var bad []string
	fm := s.fm
	if !fm.present {
		return []string{"missing frontmatter (the file must start with ---)"}
	}
	name, ok := fm.get("name")
	switch {
	case !ok || strings.TrimSpace(name.s) == "":
		bad = append(bad, "name is missing")
	case !nameRe.MatchString(name.s):
		bad = append(bad, fmt.Sprintf("name %q must match %s", name.s, nameRe))
	case name.s != filepath.Base(s.dir):
		bad = append(bad, fmt.Sprintf("name %q does not match its directory %q", name.s, filepath.Base(s.dir)))
	}
	desc, ok := fm.get("description")
	d := strings.TrimSpace(desc.s)
	switch {
	case !ok || d == "":
		bad = append(bad, "description is missing")
	case strings.IndexFunc(d, unicode.IsControl) >= 0:
		bad = append(bad, "description must be one line")
	case utf8.RuneCountInString(d) > 1024:
		bad = append(bad, fmt.Sprintf("description is %d characters, over 1024", utf8.RuneCountInString(d)))
	}
	if v, ok := fm.get("kind"); ok && !slices.Contains(Kinds, v.s) {
		bad = append(bad, fmt.Sprintf("kind %q must be one of %s", v.s, strings.Join(Kinds, ", ")))
	}
	if v, ok := fm.get("tier"); ok && !slices.Contains(Tiers, v.s) {
		bad = append(bad, fmt.Sprintf("tier %q must be one of %s", v.s, strings.Join(Tiers, ", ")))
	}
	return bad
}
