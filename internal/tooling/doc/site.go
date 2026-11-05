package doc

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
)

// siteFS holds the readable site sources. They are embedded so the generator
// and the published site need no runtime files.
//
//go:embed site/templates/*.html site/assets/*
var siteFS embed.FS

type siteRenderer struct {
	page      *template.Template
	index     *template.Template
	css       template.CSS
	js        template.JS
	themeInit template.JS
}

func newSiteRenderer() (*siteRenderer, error) {
	css, err := siteFS.ReadFile("site/assets/site.css")
	if err != nil {
		return nil, fmt.Errorf("read doc CSS: %w", err)
	}
	js, err := siteFS.ReadFile("site/assets/site.js")
	if err != nil {
		return nil, fmt.Errorf("read doc JS: %w", err)
	}
	themeInit, err := siteFS.ReadFile("site/assets/theme-init.js")
	if err != nil {
		return nil, fmt.Errorf("read doc theme init: %w", err)
	}
	page, err := parseSiteTemplate("page.html")
	if err != nil {
		return nil, err
	}
	index, err := parseSiteTemplate("index.html")
	if err != nil {
		return nil, err
	}
	return &siteRenderer{
		page:      page,
		index:     index,
		css:       template.CSS(css),
		js:        template.JS(js),
		themeInit: template.JS(themeInit),
	}, nil
}

// parseSiteTemplate parses one page variant. The layout and partials are shared;
// page.html and index.html each define "main", so they must be parsed separately.
func parseSiteTemplate(main string) (*template.Template, error) {
	t, err := template.ParseFS(siteFS,
		"site/templates/layout.html",
		"site/templates/partials.html",
		"site/templates/icons.html",
		"site/templates/"+main,
	)
	if err != nil {
		return nil, fmt.Errorf("parse doc template %s: %w", main, err)
	}
	return t, nil
}

func (r *siteRenderer) render(t *template.Template, data pageData) (string, error) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		return "", fmt.Errorf("render doc page: %w", err)
	}
	return buf.String(), nil
}
