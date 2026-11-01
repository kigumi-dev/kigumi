package get

import (
	"sort"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

// nativeSnapshot is one dependency's native declarations at a point in
// time, for the before/after lock diff's JSON.
type nativeSnapshot struct {
	Links   []linkJSON   `json:"links,omitempty"`
	Sources []sourceJSON `json:"sources,omitempty"`
}

type linkJSON struct {
	Library string `json:"library,omitempty"`
	Search  string `json:"search,omitempty"`
}

type sourceJSON struct {
	Path string `json:"path,omitempty"`
	Asm  bool   `json:"asm,omitempty"`
}

func snapshotJSON(mf driver.ModFile) *nativeSnapshot {
	s := &nativeSnapshot{}
	for _, l := range mf.Links {
		s.Links = append(s.Links, linkJSON{Library: l.Library, Search: l.Search})
	}
	for _, src := range mf.Sources {
		s.Sources = append(s.Sources, sourceJSON{Path: src.Path, Asm: src.Asm})
	}
	return s
}

// nativeChange is one dependency whose native declarations changed across
// `kigumi get`; Lines is the human summary, not part of the machine-readable diff.
type nativeChange struct {
	Module string          `json:"module"`
	Before *nativeSnapshot `json:"before,omitempty"`
	After  *nativeSnapshot `json:"after,omitempty"`
	Lines  []string        `json:"-"`
}

// snapshotNative reads the native declarations the current lock records,
// before `kigumi get` fetches or updates anything.
func snapshotNative(lock driver.Lock) map[string]driver.ModFile {
	out := map[string]driver.ModFile{}
	for name, entry := range lock.Entries {
		dir, err := driver.CacheDir(session.CompilerOptions(), driver.Require{Name: name, Version: entry.Version})
		if err != nil {
			continue
		}
		if mf, ok, err := driver.ReadModFile(dir); err == nil && ok {
			out[name] = mf
		}
	}
	return out
}

// diffNative compares the native declarations before and after a fetch,
// skipping dependencies whose Links and Sources did not change.
func diffNative(before, after map[string]driver.ModFile) []nativeChange {
	names := map[string]bool{}
	for n := range before {
		names[n] = true
	}
	for n := range after {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	var out []nativeChange
	for _, n := range sorted {
		b, hasB := before[n]
		a, hasA := after[n]
		if hasB && hasA && linksEqual(b.Links, a.Links) && sourcesEqual(b.Sources, a.Sources) {
			continue
		}
		if !hasB && !hasA {
			continue
		}
		c := nativeChange{Module: n, Lines: describeChange(hasB, hasA, b, a)}
		if hasB {
			c.Before = snapshotJSON(b)
		}
		if hasA {
			c.After = snapshotJSON(a)
		}
		out = append(out, c)
	}
	return out
}

func linksEqual(a, b []driver.Link) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sourcesEqual(a, b []driver.NativeSource) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
