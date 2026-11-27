package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"apex/internal/exit"
)

func touch(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIDsIgnoresUnrelatedNames(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"002.json", "010.json", "001.md", ".gitignore", "handoff.md", "per-ticket", "concurrent", "003.brief.md"} {
		touch(t, dir, n)
	}
	if err := os.Mkdir(filepath.Join(dir, "004"), 0o755); err != nil {
		t.Fatal(err)
	}
	ids, err := IDs(dir, ".json")
	if err != nil || !slices.Equal(ids, []int{2, 10}) {
		t.Fatalf("IDs = %v, %v; want [2 10]", ids, err)
	}
	if ids, err := IDs(filepath.Join(dir, "missing"), ".json"); err != nil || ids != nil {
		t.Fatalf("missing dir: %v, %v", ids, err)
	}
}

func TestIDsRejectsNonCanonical(t *testing.T) {
	for _, n := range []string{"0005.md", "000.md", "99999999999999999999.md"} {
		dir := t.TempDir()
		touch(t, dir, n)
		if _, err := IDs(dir, ".md"); !errors.Is(err, exit.Corrupt) {
			t.Errorf("%s: err = %v, want Corrupt", n, err)
		}
	}
	dir := t.TempDir()
	touch(t, dir, "1000.md")
	if ids, err := IDs(dir, ".md"); err != nil || !slices.Equal(ids, []int{1000}) {
		t.Errorf("1000.md: %v, %v", ids, err)
	}
}

func TestWriteLinkRefusesTakenName(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, "001.json", []byte("a"), os.Link); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, "001.json", []byte("b"), os.Link); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second link: err = %v, want ErrExist", err)
	}
	if err := Write(dir, "001.json", []byte("c"), os.Rename); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "001.json")); string(data) != "c" {
		t.Fatalf("content = %q, want c", data)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}
