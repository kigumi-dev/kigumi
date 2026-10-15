package schema

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"kigumi/internal/sem"
	"kigumi/internal/version"
)

// GeneratorVersion must be bumped whenever any derive generator's output changes; TestGeneratorVersionPinsOutput fails until it is.
const GeneratorVersion = 1

// DeriveCache memoizes generated derive source on disk, keyed by the generator version and the declaration's Schema IR.
type DeriveCache struct {
	Dir          string
	hits, misses atomic.Int64
}

func (c *DeriveCache) Hits() int   { return int(c.hits.Load()) }
func (c *DeriveCache) Misses() int { return int(c.misses.Load()) }

// Generate matches sem.DeriveGenerator. A missing, unreadable or corrupt entry is regenerated; a failed write is ignored.
func (c *DeriveCache) Generate(res *sem.Result, req sem.DeriveRequest) (string, error) {
	d, gen, err := deriveInput(res, req)
	if err != nil {
		return "", err
	}
	path := filepath.Join(c.Dir, unitKey(gen.ID(), d))
	if text, ok := readUnit(path); ok {
		c.hits.Add(1)
		return text, nil
	}
	c.misses.Add(1)
	text, err := runTwice(gen, d)
	if err != nil {
		return "", err
	}
	writeUnit(path, text)
	return text, nil
}

func unitKey(id GeneratorID, d *Declaration) string {
	return digest(struct {
		GeneratorVersion int
		SchemaMajor      int
		SchemaMinor      int
		Generator        GeneratorID
		Decl             *Declaration
	}{GeneratorVersion, version.SchemaIRFormatMajor, version.SchemaIRFormatMinor, id, d})
}

// An entry is the text's digest, a newline, then the text, so a truncated write reads as a miss.
func readUnit(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	sum, text, ok := strings.Cut(string(data), "\n")
	if !ok || sum != digestBytes([]byte(text)) {
		return "", false
	}
	return text, true
}

func writeUnit(path, text string) {
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".unit-*")
	if err != nil {
		return
	}
	_, werr := f.WriteString(digestBytes([]byte(text)) + "\n" + text)
	cerr := f.Close()
	if werr != nil || cerr != nil || os.Rename(f.Name(), path) != nil {
		os.Remove(f.Name())
	}
}
