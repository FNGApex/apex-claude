// Package worker manages an Orchestrator's workers: each is a temporary git worktree on its own branch under
// <main>/.apex/workers/<id>, described by the manifest <id>.json beside it. An adopted (external) worker tracks a
// worktree or branch some other tool made, wherever it lives. The manifest records intent; git is asked for the live
// state every time.
package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"apex/internal/exit"
	"apex/internal/layout"
	"apex/internal/ledger"
	"apex/internal/store"
)

const (
	StateActive  = "active"
	StateMerged  = "merged"
	StateRemoved = "removed"
)

// States are every manifest state, in lifecycle order.
var States = []string{StateActive, StateMerged, StateRemoved}

type Manifest struct {
	ID        string   `json:"id"`
	Slug      string   `json:"slug"`
	State     string   `json:"state"`
	Branch    string   `json:"branch"`
	Base      string   `json:"base"`
	BaseSHA   string   `json:"base_sha"`
	Skills    []string `json:"skills"`
	Item      string   `json:"item,omitempty"`
	Created   string   `json:"created"`
	Merged    string   `json:"merged,omitempty"`
	MergedSHA string   `json:"merged_sha,omitempty"`
	Removed   string   `json:"removed,omitempty"`
	External  bool     `json:"external,omitempty"` // adopted: apex did not create the worktree or branch
	Worktree  string   `json:"worktree,omitempty"` // external only; empty when only the branch was adopted
}

// Live is what git says about a worker now. Worktree is ok, missing (no directory), drift (another branch checked
// out) or branch (an adopted branch with no worktree). Ahead and Behind compare the worker branch with its base.
type Live struct {
	Worktree string `json:"worktree"`
	Head     string `json:"head"`
	Dirty    int    `json:"dirty"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
}

var (
	slugRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	itemRe  = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*/)?[0-9]+$`)
	skillRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

// badRef rejects an adopted branch name apex could not have recorded: empty, whitespace, control chars or "..".
func badRef(s string) bool {
	return s == "" || strings.Contains(s, "..") || strings.ContainsFunc(s, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	})
}

func CheckSlug(s string) error {
	if !slugRe.MatchString(s) {
		return fmt.Errorf("%w: slug must match %s, got %q", exit.Usage, slugRe, s)
	}
	return nil
}

// CheckItem validates the optional ledger item a worker serves: an id, or agent/id.
func CheckItem(s string) error {
	if s != "" && !itemRe.MatchString(s) {
		return fmt.Errorf("%w: item must match %s, got %q", exit.Usage, itemRe, s)
	}
	return nil
}

// ParseSkills splits a comma-separated skill list, validating each name.
func ParseSkills(s string) ([]string, error) {
	out := []string{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, sk := range strings.Split(s, ",") {
		sk = strings.TrimSpace(sk)
		if !skillRe.MatchString(sk) {
			return nil, fmt.Errorf("%w: skill must match %s, got %q", exit.Usage, skillRe, sk)
		}
		out = append(out, sk)
	}
	return out, nil
}

// ParseID accepts a worker id as digits only.
func ParseID(s string) (int, error) { return ledger.ParseID(s) }

type Manager struct {
	root string
	dir  string
	now  func() time.Time
}

// New manages the workers of the main checkout root.
func New(root string) *Manager {
	return &Manager{root: root, dir: layout.Workers(root), now: time.Now}
}

// Worktree is the derived checkout path of a worker.
func (m *Manager) Worktree(id string) string { return filepath.Join(m.dir, id) }

// WorktreePath is where the worker's checkout is: the derived path, or for an adopted worker the recorded one ("" when
// only its branch was adopted).
func (m *Manager) WorktreePath(mf Manifest) string {
	if !mf.External {
		return m.Worktree(mf.ID)
	}
	if mf.Worktree == "" {
		return ""
	}
	return filepath.Clean(mf.Worktree)
}

func (m *Manager) stamp() string { return m.now().UTC().Format(time.RFC3339) }

func (m *Manager) Spawn(dir, slug, base string, skills []string, item string) (Manifest, error) {
	if err := CheckSlug(slug); err != nil {
		return Manifest{}, err
	}
	if err := CheckItem(item); err != nil {
		return Manifest{}, err
	}
	base, sha, err := m.resolveBase(dir, base)
	if err != nil {
		return Manifest{}, err
	}
	mf, err := m.create(Manifest{Slug: slug, Base: base, BaseSHA: sha, Skills: skills, Item: item})
	if err != nil {
		return Manifest{}, err
	}
	if _, err := git(m.root, "worktree", "add", "-q", "-b", mf.Branch, m.Worktree(mf.ID), sha); err != nil {
		os.Remove(filepath.Join(m.dir, mf.ID+".json"))
		return Manifest{}, err
	}
	return mf, nil
}

// resolveBase defaults base to dir's current branch and returns it with its commit.
func (m *Manager) resolveBase(dir, base string) (string, string, error) {
	if base == "" {
		cur, err := git(dir, "branch", "--show-current")
		if err != nil {
			return "", "", err
		}
		if cur == "" {
			return "", "", errors.New("HEAD is detached; pass --base <branch>")
		}
		base = cur
	}
	sha, err := git(m.root, "rev-parse", "-q", "--verify", "refs/heads/"+base+"^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("%w: no local branch %q", exit.NotFound, base)
	}
	return base, sha, nil
}

// create writes mf as a new active manifest under the next free id; a spawned worker's branch derives from that id.
func (m *Manager) create(mf Manifest) (Manifest, error) {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return Manifest{}, err
	}
	// The worktrees sit inside the main checkout; the ignore file keeps them out of its status.
	ignore := filepath.Join(m.dir, ".gitignore")
	if _, err := os.Stat(ignore); os.IsNotExist(err) {
		if err := os.WriteFile(ignore, []byte("*\n"), 0o644); err != nil {
			return Manifest{}, err
		}
	}
	if mf.Skills == nil {
		mf.Skills = []string{}
	}
	mf.State, mf.Created = StateActive, m.stamp()
	for {
		ids, err := store.IDs(m.dir, ".json")
		if err != nil {
			return Manifest{}, err
		}
		next := 1
		if len(ids) > 0 {
			next = ids[len(ids)-1] + 1
		}
		mf.ID = fmt.Sprintf("%03d", next)
		if !mf.External {
			mf.Branch = "apex/" + mf.ID + "-" + mf.Slug
		}
		err = m.write(mf, os.Link)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return Manifest{}, err
		}
		// A concurrent spawn won the id; retry unless an unlisted name holds it, which would spin forever.
		if ids, err = store.IDs(m.dir, ".json"); err != nil {
			return Manifest{}, err
		}
		if !slices.Contains(ids, next) {
			return Manifest{}, fmt.Errorf("%w: %s is taken by a file that is not a manifest", exit.Corrupt,
				filepath.Join(m.dir, store.Name(next, ".json")))
		}
	}
	return mf, nil
}

var slugJunk = regexp.MustCompile(`[^a-z0-9]+`)

// Adopt tracks an existing worktree or local branch (target: a branch name, refs/heads/ optional, or a worktree path)
// as an external worker. Apex never created it, so removing the worker later leaves it in place unless purged.
func (m *Manager) Adopt(dir, target, slug, base string, skills []string, item string) (Manifest, error) {
	if err := CheckItem(item); err != nil {
		return Manifest{}, err
	}
	if slug != "" {
		if err := CheckSlug(slug); err != nil {
			return Manifest{}, err
		}
	}
	if target == "" || strings.ContainsFunc(target, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return Manifest{}, fmt.Errorf("%w: adopt needs a worktree path or branch, got %q", exit.Usage, target)
	}
	base, _, err := m.resolveBase(dir, base)
	if err != nil {
		return Manifest{}, err
	}
	wts, err := m.worktrees()
	if err != nil {
		return Manifest{}, err
	}
	name := strings.TrimPrefix(target, "refs/heads/")
	wtPath, branch, found := "", "", false
	for _, w := range wts {
		if w.branch != "" && w.branch == name {
			wtPath, branch, found = w.path, w.branch, true
			break
		}
	}
	if !found {
		p := target
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		p = canon(p)
		for _, w := range wts {
			if samePath(canon(w.path), p) {
				if w.branch == "" {
					return Manifest{}, fmt.Errorf("%w: worktree %s has detached HEAD", exit.Transition, w.path)
				}
				wtPath, branch, found = w.path, w.branch, true
				break
			}
		}
	}
	if !found {
		if badRef(name) {
			return Manifest{}, fmt.Errorf("%w: no worktree or local branch %q", exit.NotFound, target)
		}
		if _, err := git(m.root, "rev-parse", "-q", "--verify", "refs/heads/"+name+"^{commit}"); err != nil {
			return Manifest{}, fmt.Errorf("%w: no worktree or local branch %q", exit.NotFound, target)
		}
		branch = name
	}
	switch {
	case wtPath != "" && samePath(canon(wtPath), canon(m.root)):
		return Manifest{}, fmt.Errorf("%w: cannot adopt the main checkout", exit.Transition)
	case branch == base:
		return Manifest{}, fmt.Errorf("%w: cannot adopt base branch %s", exit.Transition, base)
	}
	all, err := m.List()
	if err != nil {
		return Manifest{}, err
	}
	for _, mf := range all {
		if mf.State == StateActive && mf.Branch == branch {
			return Manifest{}, fmt.Errorf("%w: branch %s is already tracked by active worker %s", exit.Transition,
				branch, mf.ID)
		}
	}
	if slug == "" {
		s := strings.TrimPrefix(strings.TrimPrefix(branch, "t3/"), "apex/")
		s = strings.Trim(slugJunk.ReplaceAllString(strings.ToLower(s), "-"), "-")
		if len(s) > 40 {
			s = strings.TrimRight(s[:40], "-")
		}
		if CheckSlug(s) != nil {
			return Manifest{}, fmt.Errorf("%w: cannot derive a slug from branch %q; pass --slug", exit.Usage, branch)
		}
		slug = s
	}
	sha, err := git(m.root, "merge-base", "refs/heads/"+base, "refs/heads/"+branch)
	if err != nil { // unrelated histories
		if sha, err = git(m.root, "rev-parse", "-q", "--verify", "refs/heads/"+base+"^{commit}"); err != nil {
			return Manifest{}, err
		}
	}
	return m.create(Manifest{Slug: slug, Branch: branch, Base: base, BaseSHA: sha, Skills: skills, Item: item,
		External: true, Worktree: wtPath})
}

func canon(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (m *Manager) write(mf Manifest, place func(oldpath, newpath string) error) error {
	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	return store.Write(m.dir, mf.ID+".json", append(data, '\n'), place)
}

// Get reads one manifest strictly: the binary wrote it, so anything it would not have written is corruption.
func (m *Manager) Get(id int) (Manifest, error) {
	name := store.Name(id, ".json")
	data, err := os.ReadFile(filepath.Join(m.dir, name))
	if os.IsNotExist(err) {
		return Manifest{}, fmt.Errorf("%w: worker %03d", exit.NotFound, id)
	}
	if err != nil {
		return Manifest{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var mf Manifest
	if err := dec.Decode(&mf); err != nil {
		return Manifest{}, fmt.Errorf("%w: %s: %v", exit.Corrupt, name, err)
	}
	switch {
	case dec.More():
		err = errors.New("trailing data")
	case mf.ID != fmt.Sprintf("%03d", id):
		err = fmt.Errorf("has id %q", mf.ID)
	case !slices.Contains(States, mf.State):
		err = fmt.Errorf("bad state %q", mf.State)
	case CheckSlug(mf.Slug) != nil:
		err = fmt.Errorf("bad slug %q", mf.Slug)
	case !mf.External && mf.Worktree != "":
		err = errors.New("worktree recorded for a worker apex created")
	case !mf.External && mf.Branch != "apex/"+mf.ID+"-"+mf.Slug:
		err = fmt.Errorf("branch %q does not match id and slug", mf.Branch)
	case mf.External && badRef(mf.Branch):
		err = fmt.Errorf("bad branch %q", mf.Branch)
	case mf.Base == "" || mf.BaseSHA == "":
		err = errors.New("missing base")
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %s %v", exit.Corrupt, name, err)
	}
	return mf, nil
}

// List returns every worker by id.
func (m *Manager) List() ([]Manifest, error) {
	ids, err := store.IDs(m.dir, ".json")
	if err != nil {
		return nil, err
	}
	out := make([]Manifest, 0, len(ids))
	for _, id := range ids {
		mf, err := m.Get(id)
		if err != nil {
			return nil, err
		}
		out = append(out, mf)
	}
	return out, nil
}

// Live asks git for the worker's state. A removed worker has none.
func (m *Manager) Live(mf Manifest) (Live, error) {
	var lv Live
	wt := m.WorktreePath(mf)
	if mf.External && wt == "" {
		// Branch-only adoption: follow the branch into a worktree if one has it checked out now.
		p, err := m.checkoutOf(mf.Branch)
		if err != nil && !errors.Is(err, exit.Transition) {
			return lv, err
		}
		wt = p
	}
	if wt == "" {
		lv.Worktree = "branch"
		if sha, err := git(m.root, "rev-parse", "-q", "--verify", "refs/heads/"+mf.Branch); err == nil {
			lv.Head = sha
		}
	} else if _, err := os.Stat(wt); err != nil {
		if !os.IsNotExist(err) {
			return lv, err
		}
		lv.Worktree = "missing"
	} else {
		out, err := git(wt, "status", "--porcelain=v2", "--branch")
		if err != nil {
			return lv, err
		}
		lv.Worktree = "drift"
		for _, line := range strings.Split(out, "\n") {
			switch {
			case line == "":
			case strings.HasPrefix(line, "# branch.oid "):
				lv.Head = strings.TrimPrefix(line, "# branch.oid ")
			case line == "# branch.head "+mf.Branch:
				lv.Worktree = "ok"
			case !strings.HasPrefix(line, "#"):
				lv.Dirty++
			}
		}
	}
	if _, err := git(m.root, "rev-parse", "-q", "--verify", "refs/heads/"+mf.Branch); err != nil {
		return lv, nil // branch gone: nothing ahead to lose
	}
	base := "refs/heads/" + mf.Base
	if _, err := git(m.root, "rev-parse", "-q", "--verify", base); err != nil {
		base = mf.BaseSHA
	}
	out, err := git(m.root, "rev-list", "--left-right", "--count", base+"...refs/heads/"+mf.Branch)
	if err != nil {
		return lv, err
	}
	if f := strings.Fields(out); len(f) == 2 {
		lv.Behind, _ = strconv.Atoi(f[0])
		lv.Ahead, _ = strconv.Atoi(f[1])
	}
	return lv, nil
}

// ConflictError is a merge that stopped on conflicts; the merge has been aborted.
type ConflictError struct {
	ID    string
	Files []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("worker %s conflicts in %s; merge aborted", e.ID, strings.Join(e.Files, ", "))
}

func (e *ConflictError) Unwrap() error { return exit.Conflict }

// Merge merges a clean worker branch with commits into its base, in the checkout that has the base checked out.
func (m *Manager) Merge(id int, msg string) (Manifest, error) {
	mf, err := m.Get(id)
	if err != nil {
		return mf, err
	}
	if mf.State == StateRemoved {
		return mf, fmt.Errorf("%w: worker %s is removed", exit.Transition, mf.ID)
	}
	lv, err := m.Live(mf)
	if err != nil {
		return mf, err
	}
	switch {
	case lv.Worktree != "ok" && !(mf.External && (lv.Worktree == "missing" || lv.Worktree == "branch")):
		return mf, fmt.Errorf("%w: worker %s worktree is %s", exit.Transition, mf.ID, lv.Worktree)
	case lv.Dirty > 0:
		return mf, fmt.Errorf("%w: worker %s has %d uncommitted change(s)", exit.Transition, mf.ID, lv.Dirty)
	case lv.Ahead == 0:
		return mf, fmt.Errorf("%w: worker %s has no commits to merge", exit.Transition, mf.ID)
	}
	w, err := m.checkoutOf(mf.Base)
	if err != nil {
		return mf, err
	}
	if _, err := git(w, "diff", "--cached", "--quiet"); err != nil {
		if exitStatus(err) == 1 {
			return mf, fmt.Errorf("%w: %s has staged changes; commit or unstage them first", exit.Transition, w)
		}
		return mf, err
	}
	if msg == "" {
		msg = fmt.Sprintf("Merge worker %s (%s) into %s", mf.ID, mf.Slug, mf.Base)
	}
	if _, merr := git(w, "merge", "--no-ff", "--no-edit", "-m", msg, "refs/heads/"+mf.Branch); merr != nil {
		if _, err := git(w, "rev-parse", "-q", "--verify", "MERGE_HEAD"); err != nil {
			// No merge in progress: git refused to start, typically over local edits the merge would overwrite.
			return mf, fmt.Errorf("%w: %v", exit.Transition, merr)
		}
		out, _ := git(w, "diff", "--name-only", "--diff-filter=U")
		files := strings.Fields(out)
		if _, err := git(w, "merge", "--abort"); err != nil {
			return mf, fmt.Errorf("merge of worker %s conflicted and the abort failed: %v", mf.ID, err)
		}
		return mf, &ConflictError{ID: mf.ID, Files: files}
	}
	sha, err := git(w, "rev-parse", "HEAD")
	if err != nil {
		return mf, err
	}
	mf.State, mf.MergedSHA, mf.Merged = StateMerged, sha, m.stamp()
	return mf, m.write(mf, os.Rename)
}

type checkout struct{ path, branch string } // branch is "" when detached

// worktrees lists every worktree of the repo with the branch it has checked out.
func (m *Manager) worktrees() ([]checkout, error) {
	out, err := git(m.root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var all []checkout
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			all = append(all, checkout{path: filepath.Clean(p)})
		} else if b, ok := strings.CutPrefix(line, "branch refs/heads/"); ok && len(all) > 0 {
			all[len(all)-1].branch = b
		}
	}
	return all, nil
}

// checkoutOf finds the worktree that has branch checked out.
func (m *Manager) checkoutOf(branch string) (string, error) {
	wts, err := m.worktrees()
	if err != nil {
		return "", err
	}
	for _, w := range wts {
		if w.branch == branch {
			return w.path, nil
		}
	}
	return "", fmt.Errorf("%w: no worktree has %s checked out; check out %s in a worktree first", exit.Transition,
		branch, branch)
}

// Remove deletes a worker's worktree and branch. Without force it refuses to lose uncommitted or unmerged work. An
// adopted worker is only untracked, its worktree and branch left to the tool that made them, unless purge.
func (m *Manager) Remove(id int, force, purge bool) (Manifest, error) {
	mf, err := m.Get(id)
	if err != nil {
		return mf, err
	}
	if mf.State == StateRemoved {
		return mf, fmt.Errorf("%w: worker %s is already removed", exit.Transition, mf.ID)
	}
	lv, err := m.Live(mf)
	if err != nil && !force {
		return mf, err
	}
	if !force && (lv.Dirty > 0 || lv.Ahead > 0) {
		return mf, fmt.Errorf("%w: worker %s has %d uncommitted change(s) and %d unmerged commit(s); pass --force to drop them",
			exit.Transition, mf.ID, lv.Dirty, lv.Ahead)
	}
	if mf.External && !purge {
		if _, err := git(m.root, "worktree", "prune"); err != nil {
			return mf, err
		}
		mf.State, mf.Removed = StateRemoved, m.stamp()
		return mf, m.write(mf, os.Rename)
	}
	wt := m.WorktreePath(mf)
	args := []string{"worktree", "prune"}
	if _, err := os.Stat(wt); wt != "" && err == nil {
		args = []string{"worktree", "remove", wt}
		if force {
			args = []string{"worktree", "remove", "--force", wt}
		}
	}
	if _, err := git(m.root, args...); err != nil {
		return mf, err
	}
	if _, err := git(m.root, "rev-parse", "-q", "--verify", "refs/heads/"+mf.Branch); err == nil {
		if _, err := git(m.root, "branch", "-D", mf.Branch); err != nil {
			return mf, err
		}
	}
	mf.State, mf.Removed = StateRemoved, m.stamp()
	return mf, m.write(mf, os.Rename)
}

// Prune removes every merged worker that holds nothing unmerged, and returns those it removed and those it skipped.
func (m *Manager) Prune() (removed, skipped []Manifest, err error) {
	all, err := m.List()
	if err != nil {
		return nil, nil, err
	}
	for _, mf := range all {
		if mf.State != StateMerged {
			continue
		}
		id, _ := strconv.Atoi(mf.ID)
		done, err := m.Remove(id, false, false)
		if errors.Is(err, exit.Transition) {
			skipped = append(skipped, mf)
			continue
		}
		if err != nil {
			return removed, skipped, err
		}
		removed = append(removed, done)
	}
	if _, err := git(m.root, "worktree", "prune"); err != nil {
		return removed, skipped, err
	}
	return removed, skipped, nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", &gitError{args: args, err: err, stderr: strings.TrimSpace(stderr.String() + "\n" + string(out))}
	}
	return strings.TrimSpace(string(out)), nil
}

type gitError struct {
	args   []string
	err    error
	stderr string
}

func (e *gitError) Error() string {
	return fmt.Sprintf("git %s: %v: %s", strings.Join(e.args, " "), e.err, e.stderr)
}

func (e *gitError) Unwrap() error { return e.err }

func exitStatus(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
