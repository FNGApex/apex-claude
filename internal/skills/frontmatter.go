package skills

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// maxHead bounds how much of a SKILL.md is read to index it.
const maxHead = 64 << 10

// value is one frontmatter value: a scalar, a list, or (for metadata only) a nested map.
type value struct {
	s      string
	list   []string
	isList bool
	sub    map[string]value
}

type frontmatter struct {
	present bool
	fields  map[string]value
	body    string // the part of the read head after the frontmatter
}

// get prefers a top-level field and falls back to metadata.<key>.
func (f frontmatter) get(key string) (value, bool) {
	if v, ok := f.fields[key]; ok {
		return v, true
	}
	v, ok := f.fields["metadata"].sub[key]
	return v, ok
}

// parseFrontmatter reads the YAML-ish head of a SKILL.md leniently: top-level scalars, quoted strings, | and > blocks,
// inline and block lists, and a one-level metadata map. It is not a YAML parser; anything else is ignored.
func parseFrontmatter(r io.Reader) frontmatter {
	sc := bufio.NewScanner(io.LimitReader(r, maxHead))
	sc.Buffer(make([]byte, 0, 4096), maxHead)
	var lines []string
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	if len(lines) > 0 {
		lines[0] = strings.TrimPrefix(lines[0], string(rune(0xfeff)))
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{fields: map[string]value{}, body: strings.Join(lines, "\n")}
	}
	for i := 1; i < len(lines); i++ {
		if t := strings.TrimSpace(lines[i]); t == "---" || t == "..." {
			return frontmatter{present: true, fields: parseMap(lines[1:i], true), body: strings.Join(lines[i+1:], "\n")}
		}
	}
	// Unterminated: no frontmatter.
	return frontmatter{fields: map[string]value{}, body: strings.Join(lines, "\n")}
}

func indent(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }

func blank(s string) bool { t := strings.TrimSpace(s); return t == "" || strings.HasPrefix(t, "#") }

// dedent strips the smallest indent of the non-blank lines.
func dedent(lines []string) []string {
	n := -1
	for _, l := range lines {
		if !blank(l) && (n < 0 || indent(l) < n) {
			n = indent(l)
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if len(l) >= n && n > 0 {
			l = l[n:]
		}
		out[i] = l
	}
	return out
}

// parseMap reads keys at indent 0; each key's value may continue on the more-indented lines after it.
func parseMap(lines []string, top bool) map[string]value {
	m := map[string]value{}
	for i := 0; i < len(lines); {
		line := lines[i]
		i++
		if blank(line) || indent(line) > 0 {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		k = strings.TrimSpace(k)
		if !ok || k == "" || strings.ContainsAny(k, " \t") {
			continue
		}
		j := i
		// YAML lets a block list sit at its key's own indent.
		for j < len(lines) && (strings.TrimSpace(lines[j]) == "" || indent(lines[j]) > 0 ||
			lines[j] == "-" || strings.HasPrefix(lines[j], "- ")) {
			j++
		}
		child := dedent(lines[i:j])
		i = j
		v = strings.TrimSpace(v)
		switch {
		case strings.HasPrefix(v, "|") || strings.HasPrefix(v, ">"):
			m[k] = value{s: block(child, v[0] == '>')}
		case strings.HasPrefix(v, "["):
			m[k] = value{list: splitInline(v), isList: true}
		case v != "":
			s := unquote(v)
			if v[0] != '"' && v[0] != '\'' && len(child) > 0 {
				s = strings.TrimSpace(s + " " + block(child, true)) // plain scalar continued on indented lines
			}
			m[k] = value{s: s}
		case isList(child):
			m[k] = value{list: blockList(child), isList: true}
		case top && k == "metadata":
			m[k] = value{sub: parseMap(child, false)}
		default:
			m[k] = value{}
		}
	}
	return m
}

func isList(lines []string) bool {
	for _, l := range lines {
		if !blank(l) {
			t := strings.TrimSpace(l)
			return t == "-" || strings.HasPrefix(t, "- ")
		}
	}
	return false
}

func blockList(lines []string) []string {
	out := []string{}
	for _, l := range lines {
		if indent(l) > 0 || blank(l) {
			continue
		}
		if t := strings.TrimSpace(l); t == "-" || strings.HasPrefix(t, "- ") {
			if s := unquote(strings.TrimSpace(strings.TrimPrefix(t, "-"))); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// block joins a | block's lines with newlines, or folds a > block's lines into one line per paragraph.
func block(lines []string, fold bool) string {
	if !fold {
		return strings.TrimRight(strings.Join(lines, "\n"), "\n ")
	}
	var paras []string
	var cur []string
	for _, l := range append(lines, "") {
		if t := strings.TrimSpace(l); t != "" {
			cur = append(cur, t)
		} else if len(cur) > 0 {
			paras, cur = append(paras, strings.Join(cur, " ")), nil
		}
	}
	return strings.Join(paras, "\n")
}

func splitInline(v string) []string {
	v = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(v, "["), "]"))
	out := []string{}
	if v == "" {
		return out
	}
	for _, p := range strings.Split(v, ",") {
		if s := unquote(strings.TrimSpace(p)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func unquote(v string) string {
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		if s, err := strconv.Unquote(v); err == nil {
			return s
		}
		return v[1 : len(v)-1]
	}
	return v
}
