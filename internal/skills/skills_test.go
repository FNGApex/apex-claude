package skills

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"apex/internal/layout"
)

func put(t *testing.T, root, rel, data string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel), "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFrontmatterForms(t *testing.T) {
	in := string(rune(0xfeff)) + "---\r\n" +
		"name: 'it''s'\r\n" +
		"description: >\r\n  folded\r\n  text\r\n" +
		"notes: |\n  line one\n  line two\n" +
		"tags: [a, \"b c\"]\n" +
		"triggers:\n- review a pr\n- \"audit\"\n" +
		"metadata:\n  kind: Orchestrator\n  tier: deep\n  requires:\n    - git\n  tags: ignored\n" +
		"---\nbody\n"
	fm := parseFrontmatter(strings.NewReader(in))
	if !fm.present {
		t.Fatal("frontmatter not found")
	}
	check := func(key, want string) {
		t.Helper()
		if got := scalar(fm, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	check("name", "it's")
	check("description", "folded text")
	check("notes", "line one\nline two")
	check("kind", "Orchestrator")
	check("tier", "deep")
	if got := list(fm, "tags", " ,"); !slices.Equal(got, []string{"a", "b c"}) {
		t.Errorf("tags = %q (top level must win over metadata)", got)
	}
	if got := list(fm, "triggers", ","); !slices.Equal(got, []string{"review a pr", "audit"}) {
		t.Errorf("triggers = %q", got)
	}
	if got := list(fm, "requires", ","); !slices.Equal(got, []string{"git"}) {
		t.Errorf("metadata.requires = %q", got)
	}
	if strings.TrimSpace(fm.body) != "body" {
		t.Errorf("body = %q", fm.body)
	}
	if parseFrontmatter(strings.NewReader("# no frontmatter\n")).present {
		t.Error("plain markdown reported frontmatter")
	}
}

func TestDiscoverPrecedenceDefaultsAndShadows(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	put(t, a, "eng/review", "---\nname: review\ndescription: Reviews code.\ntriggers: review a pr\n---\n")
	put(t, a, "bare", "# Bare\n\nFirst body line.\n")
	put(t, a, "eng/review/templates/inner", "---\nname: inner\n---\n") // inside a skill: not indexed
	put(t, a, ".hidden/x", "---\nname: x\n---\n")
	put(t, a, "node_modules/y", "---\nname: y\n---\n")
	put(t, a, "1/2/3/4/5/6/deep", "---\nname: deep\n---\n") // seven levels down
	shadow := put(t, b, "review", "---\nname: review\ndescription: other\n---\n")

	got, warnings := Discover([]layout.Root{{Path: a, Source: "repo"}, {Path: filepath.Join(a, "missing"), Source: "env"}, {Path: b, Source: "user"}})
	if len(warnings) != 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	var names []string
	for _, s := range got {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"bare", "review"}) {
		t.Fatalf("names = %v", names)
	}
	bare, review := got[0], got[1]
	if bare.Description != "First body line." || bare.Kind != "skill" || bare.Domain != "" {
		t.Errorf("bare = %+v", bare)
	}
	if review.Source != "repo" || review.Domain != "eng" || !slices.Equal(review.Tags, []string{"eng"}) ||
		!slices.Equal(review.Shadows, []string{shadow}) || review.Ref() != "eng/review" {
		t.Errorf("review = %+v", review)
	}
	if Find(got, "eng/review") != review || Find(got, "review") != review || Find(got, "nope") != nil {
		t.Error("Find by name or domain/name failed")
	}
}

func TestSearchScoring(t *testing.T) {
	root := t.TempDir()
	put(t, root, "pr-review", "---\nname: pr-review\ndescription: Review one pull request.\ntags: github\n---\n")
	put(t, root, "review", "---\nname: review\ndescription: Generic.\n---\n")
	put(t, root, "sign-off", "---\nname: sign-off\ndescription: End a session.\ntriggers: [wrap up the review]\n---\n")
	all, _ := Discover([]layout.Root{{Path: root, Source: "repo"}})
	hits := Search(all, "Review", 10)
	var got []string
	for _, h := range hits {
		got = append(got, h.Name)
	}
	// review: exact 8; pr-review: contains 6 + desc 2; sign-off: trigger 4.
	if !slices.Equal(got, []string{"pr-review", "review", "sign-off"}) || hits[0].Score != 8 || hits[1].Score != 8 {
		t.Fatalf("hits = %v (%+v)", got, hits)
	}
	if hits := Search(all, "github", 10); len(hits) != 1 || hits[0].Score != 5 {
		t.Fatalf("tag hit = %+v", hits)
	}
	if hits := Search(all, "review", 1); len(hits) != 1 {
		t.Fatalf("limit ignored: %d hits", len(hits))
	}
	if hits := Search(all, "zzz", 10); len(hits) != 0 {
		t.Fatalf("zero-score hits kept: %+v", hits)
	}
}

func TestCheck(t *testing.T) {
	root := t.TempDir()
	put(t, root, "good", "---\nname: good\ndescription: Fine.\nmetadata:\n  kind: protocol\n  tier: fast\n---\n")
	put(t, root, "none", "# no frontmatter\n")
	put(t, root, "wrong", "---\nname: other\ndescription: |\n  two\n  lines\nkind: wizard\ntier: ultra\n---\n")
	put(t, root, "nodesc", "---\nname: nodesc\n---\nbody line\n")
	found, _ := Walk(layout.Root{Path: root, Source: "repo"}, 0)
	problems := map[string][]string{}
	for _, s := range found {
		problems[filepath.Base(s.dir)] = Check(s)
	}
	if len(problems["good"]) != 0 {
		t.Errorf("good: %v", problems["good"])
	}
	if len(problems["none"]) != 1 {
		t.Errorf("none: %v", problems["none"])
	}
	if len(problems["wrong"]) != 4 {
		t.Errorf("wrong: want name, description, kind, tier problems, got %v", problems["wrong"])
	}
	if p := problems["nodesc"]; len(p) != 1 || !strings.Contains(p[0], "description") {
		t.Errorf("nodesc: %v (a body fallback must not satisfy check)", p)
	}
}

// The repo's own skills must pass the schema they ship.
func TestRepoSkillsPass(t *testing.T) {
	found, warnings := Walk(layout.Root{Path: filepath.Join("..", "..", "skills"), Source: "repo"}, 0)
	if len(found) == 0 || len(warnings) != 0 {
		t.Fatalf("found %d skills, warnings %v", len(found), warnings)
	}
	for _, s := range found {
		if bad := Check(s); len(bad) != 0 {
			t.Errorf("%s: %v", s.Path, bad)
		}
	}
}
