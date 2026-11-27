// Package store holds the numbered-file helpers the ledger and the worker manifests share: canonical ids and atomic
// writes.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"apex/internal/exit"
)

// Name is the canonical file name of id: zero-padded to three digits.
func Name(id int, ext string) string { return fmt.Sprintf("%03d%s", id, ext) }

// IDs returns the sorted ids of the <digits><ext> files in dir, ignoring every other name. A missing dir has no ids. A
// numbered name that is not canonical (0005.md, 000.md) is corruption: two names for one id would list it twice.
func IDs(dir, ext string) ([]int, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`^([0-9]+)` + regexp.QuoteMeta(ext) + `$`)
	var ids []int
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 || e.Name() != Name(n, ext) {
			return nil, fmt.Errorf("%w: %s is not a canonical name", exit.Corrupt, filepath.Join(dir, e.Name()))
		}
		ids = append(ids, n)
	}
	sort.Ints(ids)
	return ids, nil
}

// Write writes data to a synced, world-readable temp file in dir, then places it at dir/name with place (os.Link to
// create, failing if the name is taken; os.Rename to replace), so readers and crashes never see a partial file.
func Write(dir, name string, data []byte, place func(oldpath, newpath string) error) error {
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(0o644)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return place(tmp.Name(), filepath.Join(dir, name))
}
