package doc

import (
	"html"
	"strings"
)

// inline turns `code` spans and [text](href) links in text into HTML,
// resolving hrefs relative to selfDir.
func inline(text, selfDir string) string {
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
			mid := strings.Index(text, "](")
			end := strings.IndexByte(text, ')')
			if mid < 0 || end < mid {
				sb.WriteString(html.EscapeString(text[:1]))
				text = text[1:]
				continue
			}
			label, target := text[1:mid], text[mid+2:end]
			if href, ok := resolveHref(target, selfDir); ok {
				sb.WriteString("<a href=\"" + html.EscapeString(href) + "\">" + html.EscapeString(label) + "</a>")
			} else {
				sb.WriteString(html.EscapeString(label))
			}
			text = text[end+1:]
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
// it unsafe: only http, https and mailto stay clickable, so a doc comment can't carry a javascript: URL.
func resolveHref(target, selfDir string) (string, bool) {
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
	if strings.HasSuffix(p, ".md") {
		site := strings.TrimSuffix(p, ".md") + ".html"
		return relHref(selfDir, site) + frag, true
	}
	return target, true
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
