package doc

import (
	"html"
	"path"
	"strings"
)

// inline turns `code` spans and [text](href) links in text into HTML,
// resolving hrefs relative to selfDir. pages holds every page the site
// publishes: a relative link to anything else is rendered as plain text, so no
// shipped page carries an href that leads nowhere.
func inline(text, selfDir string, pages map[string]bool) string {
	var sb strings.Builder
	for len(text) > 0 {
		switch {
		case text[0] == '`':
			end := strings.IndexByte(text[1:], '`')
			if end < 0 {
				sb.WriteString(html.EscapeString(text))
				return sb.String()
			}
			sb.WriteString("<code>" + html.EscapeString(text[1:1+end]) + "</code>")
			text = text[end+2:]
		case text[0] == '[':
			l, ok := scanLink(text, 0)
			if !ok {
				sb.WriteString(html.EscapeString(text[:1]))
				text = text[1:]
				continue
			}
			if href, ok := resolveHref(l.dest, selfDir, pages); ok {
				sb.WriteString("<a href=\"" + html.EscapeString(href) + "\">" + html.EscapeString(l.label) + "</a>")
			} else {
				sb.WriteString(html.EscapeString(l.label))
			}
			text = text[l.end:]
		default:
			i := strings.IndexAny(text, "`[")
			if i < 0 {
				i = len(text)
			}
			sb.WriteString(html.EscapeString(text[:i]))
			text = text[i:]
		}
	}
	return sb.String()
}

// resolveHref classifies a Markdown link target into a usable href, or reports
// it unusable: only http, https and mailto stay clickable, so a doc comment
// can't carry a javascript: URL, and a relative target is clickable only when
// pages holds the page it names.
func resolveHref(target, selfDir string, pages map[string]bool) (string, bool) {
	if target == "" {
		return "", false
	}
	if target[0] == '#' {
		return target, true
	}
	if scheme, ok := uriScheme(target); ok {
		switch scheme {
		case "http", "https", "mailto":
			return target, true
		default:
			return "", false
		}
	}
	p, frag := target, ""
	if i := strings.IndexByte(target, '#'); i >= 0 {
		p, frag = target[:i], target[i:]
	}
	page, ok := linkPage(p)
	if !ok {
		return "", false
	}
	site, ok := publishedTarget(page, selfDir, pages)
	if !ok {
		return "", false
	}
	return relHref(selfDir, site+".html") + frag, true
}

// publishedTarget maps the page path a link names to the page the site serves:
// a target is read as root-relative first, then relative to selfDir so a page
// can link to its sibling package with a plain file name.
func publishedTarget(page, selfDir string, pages map[string]bool) (string, bool) {
	if pages[page] {
		return page, true
	}
	if selfDir != "" {
		if sibling := path.Join(selfDir, page); pages[sibling] {
			return sibling, true
		}
	}
	return "", false
}

// linkPage maps a relative Markdown link to the page path it names; the site
// publishes one ".html" page per page path.
func linkPage(p string) (string, bool) {
	switch {
	case strings.HasSuffix(p, ".md"):
		return strings.TrimSuffix(p, ".md"), true
	case strings.HasSuffix(p, ".html"):
		return strings.TrimSuffix(p, ".html"), true
	}
	return "", false
}

// uriScheme reports the lowercase scheme of an absolute URI per RFC 3986, or
// false when target has no scheme.
func uriScheme(target string) (string, bool) {
	for i := 0; i < len(target); i++ {
		c := target[i]
		switch {
		case c == ':':
			if i == 0 {
				return "", false
			}
			return strings.ToLower(target[:i]), true
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return "", false
		}
	}
	return "", false
}
