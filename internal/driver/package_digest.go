package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// PackageDigest hashes a parsed package's file names and contents; "" if unknown.
func (m *Module) PackageDigest(path string) string {
	p := m.Packages[path]
	if p == nil {
		return ""
	}
	names := make([]string, 0, len(p.Files))
	byName := map[string][]byte{}
	for _, t := range p.Files {
		names = append(names, t.File.Name)
		byName[t.File.Name] = t.File.Src
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		h.Write([]byte(n))
		h.Write([]byte{0})
		h.Write(byName[n])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
