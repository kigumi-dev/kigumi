package doc

import (
	"html/template"
	"path"
	"sort"
	"strings"
)

// HTML renders the Markdown pages as a static site: one page per package
// plus index.html. The site is self-contained (inline CSS and JS) so it works
// offline and from any directory depth. Every href is relative to the page
// that carries it.
//
// Markup lives in site/templates and assets in site/assets; this file only
// shapes the view data. The templates are parsed from the embedded copy and
// execute through html/template, which applies contextual escaping.
func HTML(pages map[string]string, title string) (map[string]string, error) {
	renderer, err := newSiteRenderer()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(pages))
	for p := range pages {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	indexName := IndexFileName(paths, ".html", "index.html")
	groups := groupPackages(paths)

	out := make(map[string]string, len(paths)+1)
	for _, p := range paths {
		selfDir := pageDir(p)
		body, toc := markdownPage(pages[p], selfDir)
		navGroups, navTitles := navView(groups, selfDir, p)
		html, err := renderer.render(renderer.page, pageData{
			Title:     p + " - " + title,
			SiteTitle: title,
			IndexHref: relHref(selfDir, indexName),
			Current:   p,
			NavTitles: navTitles,
			NavGroups: navGroups,
			TOC:       toc,
			Body:      template.HTML(body),
			CSS:       renderer.css,
			JS:        renderer.js,
			ThemeInit: renderer.themeInit,
		})
		if err != nil {
			return nil, err
		}
		out[p+".html"] = html
	}
	navGroups, navTitles := navView(groups, "", "")
	html, err := renderer.render(renderer.index, pageData{
		Title:        title,
		SiteTitle:    title,
		IndexHref:    relHref("", indexName),
		NavTitles:    navTitles,
		NavGroups:    navGroups,
		CSS:          renderer.css,
		JS:           renderer.js,
		ThemeInit:    renderer.themeInit,
		PackageCount: len(paths),
		IndexGroups:  indexView(groups),
	})
	if err != nil {
		return nil, err
	}
	out[indexName] = html
	return out, nil
}

// pageDir is the root-relative directory a page's file lives in; "" at the site root.
func pageDir(p string) string {
	if d := path.Dir(p); d != "." {
		return d
	}
	return ""
}

// relHref makes target (a root-relative site path, e.g. "std/array.html")
// relative to a page living in selfDir.
func relHref(selfDir, target string) string {
	if selfDir == "" {
		return target
	}
	return strings.Repeat("../", strings.Count(selfDir, "/")+1) + target
}

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

type navItemView struct {
	Path    string
	Label   string
	Href    string
	Search  string
	Current bool
}

type navGroupView struct {
	Label string
	Count int
	Items []navItemView
}

func navView(groups []packageGroup, selfDir, current string) ([]navGroupView, bool) {
	out := make([]navGroupView, 0, len(groups))
	for _, g := range groups {
		items := make([]navItemView, 0, len(g.items))
		for _, it := range g.items {
			items = append(items, navItemView{
				Path:    it.path,
				Label:   it.label,
				Href:    relHref(selfDir, it.path+".html"),
				Search:  strings.ToLower(it.path),
				Current: it.path == current,
			})
		}
		out = append(out, navGroupView{
			Label: groupLabel(g),
			Count: len(g.items),
			Items: items,
		})
	}
	return out, showGroupTitles(groups)
}

type indexItemView struct {
	Name     string
	Path     string
	Href     string
	Search   string
	ShowPath bool
}

type indexGroupView struct {
	Label string
	Count int
	Items []indexItemView
}

func indexView(groups []packageGroup) []indexGroupView {
	out := make([]indexGroupView, 0, len(groups))
	for _, g := range groups {
		items := make([]indexItemView, 0, len(g.items))
		for _, it := range g.items {
			items = append(items, indexItemView{
				Name:     it.label,
				Path:     it.path,
				Href:     it.path + ".html",
				Search:   strings.ToLower(it.path),
				ShowPath: it.label != it.path,
			})
		}
		out = append(out, indexGroupView{
			Label: groupLabel(g),
			Count: len(g.items),
			Items: items,
		})
	}
	return out
}

type tocEntry struct {
	ID    string
	Text  string
	Level int
}

type pageData struct {
	Title     string
	SiteTitle string
	IndexHref string
	Current   string

	NavTitles bool
	NavGroups []navGroupView

	TOC  []tocEntry
	Body template.HTML

	CSS       template.CSS
	JS        template.JS
	ThemeInit template.JS

	PackageCount int
	IndexGroups  []indexGroupView
}
