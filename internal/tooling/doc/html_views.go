package doc

import (
	"html/template"
	"strings"
)

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
