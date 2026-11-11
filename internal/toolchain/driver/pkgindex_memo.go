package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	buildconfig "kigumi/internal/toolchain/config"
)

const memoVersion = 4

// racyWindow keeps a file whose mtime is this close to the time it was read out of the memo: a later edit could keep the same size and mtime tick.
const racyWindow = 2 * time.Second

type memoEntry struct {
	Size    int64    `json:"size"`
	Mtime   int64    `json:"mtime"`
	Ctime   int64    `json:"ctime"`
	At      int64    `json:"at"`
	Hash    string   `json:"hash"`
	Imports []string `json:"imports,omitempty"`
}

// indexMemo remembers each file's content hash and imports under its size and mtime, so a no-op load stats files instead of reading them.
type indexMemo struct {
	path    string
	entries map[string]memoEntry
	dirty   bool
	now     func() time.Time
}

type memoFile struct {
	Version int                  `json:"version"`
	Entries map[string]memoEntry `json:"entries"`
}

func loadIndexMemo(cacheDir, root string, o buildconfig.CompilerOptions, now func() time.Time) *indexMemo {
	if cacheDir == "" {
		return nil
	}
	abs, err := o.Abs(root)
	if err != nil {
		return nil
	}
	sum := sha256.Sum256([]byte(abs))
	m := &indexMemo{path: filepath.Join(cacheDir, "index", hex.EncodeToString(sum[:8])+".json"), entries: map[string]memoEntry{}, now: now}
	var f memoFile
	if b, err := os.ReadFile(m.path); err == nil && json.Unmarshal(b, &f) == nil && f.Version == memoVersion && f.Entries != nil {
		m.entries = f.Entries
	}
	return m
}

func (m *indexMemo) lookup(abs string, size, mtime, ctime int64) (memoEntry, bool) {
	if m == nil {
		return memoEntry{}, false
	}
	e, ok := m.entries[abs]
	if !ok || e.Size != size || e.Mtime != mtime || e.Ctime != ctime || e.At-e.Mtime < int64(racyWindow) {
		return memoEntry{}, false
	}
	return e, true
}

func (m *indexMemo) put(abs string, size, mtime, ctime int64, hash string, imports []string) {
	if m == nil {
		return
	}
	m.entries[abs] = memoEntry{Size: size, Mtime: mtime, Ctime: ctime, At: m.now().UnixNano(), Hash: hash, Imports: imports}
	m.dirty = true
}

// save drops entries of files that are gone and writes the memo atomically; a failure only costs the next load a re-read.
func (m *indexMemo) save() {
	if m == nil || !m.dirty {
		return
	}
	for abs := range m.entries {
		if _, err := os.Stat(abs); err != nil {
			delete(m.entries, abs)
		}
	}
	b, err := json.Marshal(memoFile{Version: memoVersion, Entries: m.entries})
	if err != nil {
		return
	}
	writeAtomic(m.path, b)
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp.Name())
		return werr
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
