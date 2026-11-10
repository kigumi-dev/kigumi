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
	published := publishedPages(paths, indexName)

	out := make(map[string]string, len(paths)+1)
	for _, p := range paths {
		selfDir := pageDir(p)
		body, toc := markdownPage(pages[p], selfDir, published)
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

// publishedPages is the set of page paths the site serves: every package page
// plus the generated index, under the name it was given after collision
// resolution.
func publishedPages(paths []string, indexName string) map[string]bool {
	published := make(map[string]bool, len(paths)+1)
	for _, p := range paths {
		published[p] = true
	}
	published[strings.TrimSuffix(indexName, ".html")] = true
	return published
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
