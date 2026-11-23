package module

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	buildconfig "kigumi/internal/toolchain/config"
)

// Selection is the result of minimal version selection: for every
// dependency reached through Require records, the highest version any path
// asked for. Requires and By are keyed by Require.Key(): an alias when the
// Require has one, else the real module name. Versions are semver tags; a
// commit hash only agrees with an identical hash.
type Selection struct {
	Requires map[string]Require
	// By names the module that asked for the selected version.
	By map[string]string
}

// MissingError reports a module whose manifest is needed to continue the
// selection but is not in the cache yet; `kigumi get` fetches it and retries.
type MissingError struct{ Require Require }

func (e *MissingError) Error() string {
	if e.Require.Alias != "" {
		return fmt.Sprintf("dependency %s (alias %s) %s is not fetched; run `kigumi get`", e.Require.Name, e.Require.Alias, e.Require.Version)
	}
	return fmt.Sprintf("dependency %s %s is not fetched; run `kigumi get`", e.Require.Name, e.Require.Version)
}

// verKey identifies one manifest: a module name at one exact version.
type verKey struct{ name, version string }

// SelectVersions runs minimal version selection in two passes: pass 1 reads
// every reachable (name, version) manifest so the maximum per name does not
// depend on discovery order; pass 2 rebuilds the edges from the winning
// manifests only, so dependencies reachable only through a loser drop out.
func SelectVersions(o buildconfig.CompilerOptions, root ModFile, local Local) (*Selection, error) {
	max := map[string]Require{}
	by := map[string]string{}
	manifests := map[verKey]ModFile{}
	visited := map[verKey]bool{}
	var queue []Require

	claims := urlClaims{}
	visit := func(r Require, from string, isRoot bool) error {
		if err := mergeRequire(max, by, r, from); err != nil {
			return err
		}
		if err := claims.claim(r, from, isRoot); err != nil {
			return err
		}
		key := verKey{r.Name, r.Version}
		if visited[key] {
			return nil
		}
		visited[key] = true
		queue = append(queue, r)
		return nil
	}

	for _, r := range root.Requires {
		if err := visit(r, DisplayName(root), true); err != nil {
			return nil, err
		}
	}
	for len(queue) > 0 {
		r := queue[0]
		queue = queue[1:]
		dir, missing, err := DepDir(o, local, r, true)
		if err != nil {
			return nil, err
		}
		if missing {
			return nil, &MissingError{Require: r}
		}
		mf, ok, err := ReadModFile(dir)
		if err != nil {
			return nil, err
		}
		if ok && mf.Name != "" && mf.Name != r.Name {
			return nil, fmt.Errorf("dependency %s: its %s names the module %q", r.Name, ManifestName, mf.Name)
		}
		manifests[verKey{r.Name, r.Version}] = mf
		for _, sub := range mf.Requires {
			if err := visit(sub, r.Name, false); err != nil {
				return nil, err
			}
		}
	}

	root2 := DisplayName(root)
	edges := map[string][]string{}
	for _, r := range root.Requires {
		edges[root2] = append(edges[root2], r.Key())
	}
	for key, r := range max {
		mf := manifests[verKey{r.Name, r.Version}]
		for _, sub := range mf.Requires {
			edges[key] = append(edges[key], sub.Key())
		}
	}
	if chain := findCycle(edges, root2); chain != nil {
		return nil, fmt.Errorf("dependency cycle: %s", strings.Join(chain, " -> "))
	}

	reachable := map[string]bool{}
	var walk func(n string)
	walk = func(n string) {
		if reachable[n] {
			return
		}
		reachable[n] = true
		for _, m := range edges[n] {
			walk(m)
		}
	}
	walk(root2)

	sel := &Selection{Requires: map[string]Require{}, By: map[string]string{}}
	for name, r := range max {
		if reachable[name] {
			sel.Requires[name], sel.By[name] = r, by[name]
		}
	}
	if err := checkSelectionAliases(root.Name, sel); err != nil {
		return nil, err
	}
	if err := checkVersionOrigins(sel); err != nil {
		return nil, err
	}
	if err := checkVendor(local, sel); err != nil {
		return nil, err
	}
	return sel, nil
}

// checkSelectionAliases rejects an alias that overlaps a selected import
// prefix or the root module's own prefix. Plain module names may nest; only
// aliases, which are local import prefixes, have to stay disjoint.
func checkSelectionAliases(rootName string, sel *Selection) error {
	for key, r := range sel.Requires {
		if r.Alias == "" {
			continue
		}
		if rootName != "" && prefixOverlap(key, rootName) {
			return fmt.Errorf("alias %s overlaps the root module's name %s", key, rootName)
		}
		for other := range sel.Requires {
			if other != key && prefixOverlap(key, other) {
				return fmt.Errorf("alias %s overlaps selected dependency %s; aliases must be disjoint import prefixes", key, other)
			}
		}
	}
	return nil
}

// checkVersionOrigins refuses two effective keys that would share one cache
// entry (same real name and literal version) but name different sources:
// the cache cannot hold both, so selection would silently use one.
func checkVersionOrigins(sel *Selection) error {
	type origin struct {
		key, url string
		aliased  bool
	}
	seen := map[verKey]origin{}
	for _, key := range sel.Names() {
		r := sel.Requires[key]
		url := RepoURLKey(r)
		k := verKey{r.Name, r.Version}
		prev, ok := seen[k]
		if !ok {
			seen[k] = origin{key, url, r.Alias != ""}
			continue
		}
		if (prev.aliased || r.Alias != "") && prev.url != "" && url != "" && prev.url != url {
			return fmt.Errorf("%s and %s share %s %s but name different sources (%s and %s); one cache entry cannot hold both", prev.key, key, r.Name, r.Version, prev.url, url)
		}
		if r.Alias != "" {
			prev.aliased = true
			seen[k] = prev
		}
	}
	return nil
}

// checkVendor refuses a vendored tree that disagrees with what selection
// picks, so a build never silently uses a stale copy. Modules a Replace
// resolves locally are not vendored and are skipped.
func checkVendor(local Local, sel *Selection) error {
	if local.Vendor == "" {
		return nil
	}
	for _, key := range sel.Names() {
		r := sel.Requires[key]
		if _, replaced := local.Replacement(r); replaced {
			continue
		}
		e, ok := local.Vendored[r.Name]
		if !ok {
			return fmt.Errorf("%s %s is selected but not in %s; run `kigumi mod vendor`", key, r.Version, filepath.Join(local.Vendor, VendorFileName))
		}
		if want := r.Version; e.Version != want {
			return fmt.Errorf("%s is vendored at %s but selection picks %s; run `kigumi get` and `kigumi mod vendor`", key, e.Version, want)
		}
	}
	return nil
}

// findCycle walks the manifest graph from root and returns the first cycle
// as a chain ending where it began.
func findCycle(edges map[string][]string, root string) []string {
	const visiting, done = 1, 2
	state := map[string]int{}
	var path []string
	var visit func(n string) []string
	visit = func(n string) []string {
		state[n] = visiting
		path = append(path, n)
		for _, m := range edges[n] {
			switch state[m] {
			case visiting:
				for i, p := range path {
					if p == m {
						return append(append([]string{}, path[i:]...), m)
					}
				}
			case 0:
				if c := visit(m); c != nil {
					return c
				}
			}
		}
		path = path[:len(path)-1]
		state[n] = done
		return nil
	}
	return visit(root)
}

// Names lists the selected modules in a stable order.
func (s *Selection) Names() []string {
	names := make([]string, 0, len(s.Requires))
	for n := range s.Requires {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// versionNewer says whether a is a newer tag than b. Two commit hashes
// only agree when equal, and a hash never compares with a tag. v0 and v1
// share one unsuffixed module path, so they compare normally; any other
// major difference means two module paths that the manifest should have
// spelled with a `/vN` suffix.
func versionNewer(a, b Require) (bool, error) {
	if a.Version == b.Version {
		return false, nil
	}
	if IsCommitHash(a.Version) || IsCommitHash(b.Version) {
		return false, fmt.Errorf("a commit hash cannot be reconciled with another version")
	}
	ma, mb := majorOf(a.Version), majorOf(b.Version)
	if ma != mb && (ma > 1 || mb > 1) {
		return false, fmt.Errorf("different major versions cannot be mixed: %s and %s belong to different module paths (`/vN`)", a.Version, b.Version)
	}
	return CompareVersions(a.Version, b.Version) > 0, nil
}

func DisplayName(mf ModFile) string {
	if mf.Name == "" {
		return "an unnamed module"
	}
	return mf.Name
}
