package module

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"kigumi/internal/base/fsx"
)

// Lock is the selected-entry view of a decoded lock file, keyed by
// Require.Key() (the alias when one is given, else the module name). A
// Replace'd dependency has no entry. It is only a view: every entry comes
// from a Lock record of the document and the full evidence stays in
// LockState.
type Lock struct {
	Entries map[string]LockEntry
}

type LockEntry struct {
	// Name is the real module name; Alias is the import prefix the Require
	// gave it ("" for a plain name-keyed dependency).
	Name, Alias           string
	Version, Commit, Hash string
	URL                   string
}

// Require reconstructs the Require a lock entry was written for.
func (e LockEntry) Require() Require {
	return Require{Name: e.Name, Version: e.Version, Alias: e.Alias}
}

// LockView returns the selected entries of a decoded lock keyed by effective
// key. It is the read model used by the driver and CLI; a LockState writer is
// the only way to produce the document those entries came from.
func (s LockState) LockView() Lock {
	lk := Lock{Entries: make(map[string]LockEntry, len(s.Locks))}
	for _, b := range s.Locks {
		lk.Entries[b.Key()] = LockEntry{
			Name:    b.Name,
			Alias:   b.Alias,
			Version: b.SelectedVersion(),
			Commit:  b.Commit.String(),
			Hash:    b.Hash,
			URL:     b.URL,
		}
	}
	return lk
}

// ReadLock decodes root/mod.lock.kg and returns its selected entries. A
// missing file is an empty view; a headerless, empty or malformed file is an
// error. The module and script lock kinds both decode as LockKindModule.
func ReadLock(root string) (Lock, error) { return ReadLockFile(filepath.Join(root, LockName)) }

// ReadLockFile decodes the lock at path (a script's `<file>.lock.kg` sits
// next to the script) and returns its selected entries.
func ReadLockFile(path string) (Lock, error) {
	state, err := ReadLockStateFile(path, LockKindModule)
	if errors.Is(err, ErrLockMissing) {
		return Lock{Entries: map[string]LockEntry{}}, nil
	}
	if err != nil {
		return Lock{Entries: map[string]LockEntry{}}, err
	}
	return state.LockView(), nil
}

// ReadLockState decodes root/mod.lock.kg as kind. Unlike ReadLock it keeps
// the whole document and reports a missing file as ErrLockMissing.
func ReadLockState(root string, kind LockKind) (LockState, error) {
	return ReadLockStateFile(filepath.Join(root, LockName), kind)
}

// WriteLockState writes state to root/mod.lock.kg in its recorded format.
// A document may carry complete Source/Resolution evidence or only selected
// Lock records; the writer validates whichever records are present and keeps
// the header even when nothing is selected.
func WriteLockState(root string, state LockState) error {
	return WriteLockStateFile(filepath.Join(root, LockName), state)
}

// WriteLockStateFile writes a lock document to an explicit path (a script's
// lock). A zero-dependency document still keeps its header.
func WriteLockStateFile(path string, state LockState) error {
	data, err := EncodeLock(state)
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(path, data, 0o644)
}

// TreeHash digests every regular file under dir (path and content) into
// "h1:<sha256 hex>", so a tampered or truncated cache entry is detectable.
func TreeHash(dir string) (string, error) { return treeHash(dir, nil) }

// ContentTreeHash digests a module's content the way TreeHash does but
// leaves out .kigumi-origin, which names the commit a cache copy came from
// rather than belonging to the module. A vendored copy has no marker, so this
// is the hash `kigumi mod vendor` records and verifies.
func ContentTreeHash(dir string) (string, error) {
	return treeHash(dir, func(rel string) bool { return rel == OriginFileName })
}

func treeHash(dir string, skip func(rel string) bool) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if skip != nil && skip(filepath.ToSlash(rel)) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(h, "%s\x00%x\n", filepath.ToSlash(rel), sum)
		return nil
	})
	if err != nil {
		return "", err
	}
	return "h1:" + hex.EncodeToString(h.Sum(nil)), nil
}
