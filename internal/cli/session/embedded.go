package shared

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"kigumi/internal/toolchain/driver"
)

var embedded fs.FS

// SetEmbeddedStd registers the std stubs compiled into the executable; dir
// is the directory inside fsys that holds prelude/, array/ and so on.
func SetEmbeddedStd(fsys fs.FS, dir string) {
	sub, err := fs.Sub(fsys, dir)
	if err == nil {
		embedded = sub
	}
}

// embeddedStdRoot unpacks the embedded std once per content hash, re-hashing an existing directory before trust since its name is predictable.
func embeddedStdRoot() string {
	if embedded == nil {
		return ""
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	hash := embeddedHash()
	dir := filepath.Join(cache, "kigumi", "std-"+hash)
	if validStdCache(dir, hash) {
		return dir
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return ""
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), "std-")
	if err != nil {
		return ""
	}
	if err := os.CopyFS(tmp, embedded); err != nil {
		os.RemoveAll(tmp)
		return ""
	}
	return publishStdCache(tmp, dir, hash)
}

// publishStdCache moves tmp to dir, renaming a stale dir aside rather than deleting it, so a reader with it already open keeps working.
func publishStdCache(tmp, dir, hash string) string {
	if err := os.Rename(tmp, dir); err == nil {
		return dir
	}
	if validStdCache(dir, hash) {
		os.RemoveAll(tmp)
		return dir
	}
	aside := dir + ".stale-" + strconv.Itoa(os.Getpid())
	if err := os.Rename(dir, aside); err != nil {
		os.RemoveAll(tmp)
		return ""
	}
	if err := os.Rename(tmp, dir); err != nil {
		os.Rename(aside, dir) // best-effort restore
		os.RemoveAll(tmp)
		return ""
	}
	os.RemoveAll(aside)
	return dir
}

// validStdCache reports whether dir hashes to hash, holds only regular files and directories, and is not writable by anyone but its owner.
func validStdCache(dir, hash string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() || st.Mode().Perm()&0o022 != 0 {
		return false
	}
	if err := driver.CheckCacheTree(dir); err != nil {
		return false
	}
	got, err := hashTree(dir)
	return err == nil && got == hash
}

// hashTree hashes an on-disk directory the same way embeddedHash hashes
// the embedded fs.FS, so the two are comparable.
func hashTree(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeFramedEntry(h, filepath.ToSlash(rel), b)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func embeddedHash() string {
	h := sha256.New()
	fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, _ := fs.ReadFile(embedded, path)
		writeFramedEntry(h, path, b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

// writeFramedEntry length-prefixes path and content so a boundary shift between the two can never make different entries hash equal.
func writeFramedEntry(h io.Writer, path string, content []byte) {
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(path)))
	h.Write(lenBuf[:])
	io.WriteString(h, path)
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(content)))
	h.Write(lenBuf[:])
	h.Write(content)
}
