package doc

import (
	"path"
	"sort"
	"strings"
)

type packageItem struct {
	path  string // full package path, used for hrefs and filtering
	label string // path relative to its group, used as the visible label
}

type packageGroup struct {
	name  string // first path element, or "" for top-level packages
	items []packageItem
}

// groupPackages buckets packages for the sidebar and index. A single package
// stays alone (its full path is the label). Several packages that share a
// directory prefix form one group: std/array and std/crypto/subtle become
// "std" with "array" and "crypto/subtle". Otherwise the first path element is
// the group, so unrelated roots still read as a tree.
func groupPackages(paths []string) []packageGroup {
	if len(paths) <= 1 {
		g := packageGroup{}
		for _, p := range paths {
			g.items = append(g.items, packageItem{path: p, label: p})
		}
		return []packageGroup{g}
	}
	if prefix := commonDirPrefix(paths); prefix != "" {
		g := packageGroup{name: strings.TrimSuffix(prefix, "/")}
		for _, p := range paths {
			label := strings.TrimPrefix(p, prefix)
			if label == "" {
				label = path.Base(p)
			}
			g.items = append(g.items, packageItem{path: p, label: label})
		}
		return []packageGroup{g}
	}

	byName := make(map[string]*packageGroup)
	var names []string
	for _, p := range paths {
		name, label := "", p
		if i := strings.IndexByte(p, '/'); i >= 0 {
			name, label = p[:i], p[i+1:]
		}
		g := byName[name]
		if g == nil {
			g = &packageGroup{name: name}
			byName[name] = g
			names = append(names, name)
		}
		g.items = append(g.items, packageItem{path: p, label: label})
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "" {
			return true
		}
		if names[j] == "" {
			return false
		}
		return names[i] < names[j]
	})
	out := make([]packageGroup, 0, len(names))
	for _, name := range names {
		out = append(out, *byName[name])
	}
	return out
}

// commonDirPrefix returns the shared directory prefix of every package,
// including its trailing slash, or "" when there is none.
func commonDirPrefix(paths []string) string {
	if len(paths) < 2 {
		return ""
	}
	parts := strings.Split(path.Dir(paths[0]), "/")
	if len(parts) == 1 && parts[0] == "." {
		return ""
	}
	for _, p := range paths[1:] {
		other := strings.Split(path.Dir(p), "/")
		n := 0
		for n < len(parts) && n < len(other) && parts[n] == other[n] {
			n++
		}
		parts = parts[:n]
		if len(parts) == 0 {
			return ""
		}
	}
	return strings.Join(parts, "/") + "/"
}

func showGroupTitles(groups []packageGroup) bool {
	return len(groups) > 1 || (len(groups) == 1 && groups[0].name != "")
}

func groupLabel(g packageGroup) string {
	if g.name == "" {
		return "Top level"
	}
	return g.name
}
