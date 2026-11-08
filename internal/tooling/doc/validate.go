package doc

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var hrefRe = regexp.MustCompile(`href="([^"]*)"`)
var idRe = regexp.MustCompile(`\sid="([^"]*)"`)

// ValidateSite checks a generated HTML site rooted at dir: every local href
// resolves to an existing file and fragment, and no page has a duplicate id.
// External links aren't checked; resolveHref already restricts them to safe schemes.
func ValidateSite(dir string) error {
	pageIDs := map[string]map[string]bool{}
	var pages []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		rel := filepath.ToSlash(mustRel(dir, p))
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		ids, dup := pageIDSet(string(b))
		if dup != "" {
			return fmt.Errorf("%s: duplicate id %q", rel, dup)
		}
		pageIDs[rel] = ids
		pages = append(pages, rel)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(pages)
	for _, rel := range pages {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		if err := validateLinks(rel, string(b), pageIDs); err != nil {
			return err
		}
	}
	return nil
}

// unescapeFragment decodes the percent-escapes a browser puts in a fragment; a malformed one stays as written.
func unescapeFragment(frag string) string {
	dec, err := url.PathUnescape(strings.TrimPrefix(frag, "#"))
	if err != nil {
		return strings.TrimPrefix(frag, "#")
	}
	return dec
}

func pageIDSet(htmlSrc string) (map[string]bool, string) {
	ids := map[string]bool{}
	for _, m := range idRe.FindAllStringSubmatch(htmlSrc, -1) {
		if ids[m[1]] {
			return nil, m[1]
		}
		ids[m[1]] = true
	}
	return ids, ""
}

func validateLinks(rel, htmlSrc string, pageIDs map[string]map[string]bool) error {
	for _, m := range hrefRe.FindAllStringSubmatch(htmlSrc, -1) {
		href := m[1]
		if href == "" {
			continue
		}
		if _, ok := uriScheme(href); ok {
			continue
		}
		target, frag := href, ""
		if i := strings.IndexByte(href, '#'); i >= 0 {
			target, frag = href[:i], href[i:]
		}
		targetPage := rel
		if target != "" {
			targetPage = path.Join(path.Dir(rel), target)
		}
		ids, ok := pageIDs[targetPage]
		if !ok {
			return fmt.Errorf("%s: broken link %q", rel, href)
		}
		if frag != "" && !ids[strings.TrimPrefix(frag, "#")] && !ids[unescapeFragment(frag)] {
			return fmt.Errorf("%s: missing fragment %q", rel, href)
		}
	}
	return nil
}

var mdLinkRe = regexp.MustCompile(`\]\(([^)]*)\)`)

// ValidateMarkdownSite checks that every ".md" link in the plain-Markdown
// output resolves to a file the run actually produced.
func ValidateMarkdownSite(dir string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		rel := filepath.ToSlash(mustRel(dir, p))
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range mdLinkRe.FindAllStringSubmatch(string(b), -1) {
			target := m[1]
			if _, ok := uriScheme(target); ok || !strings.HasSuffix(target, ".md") {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(target))); err != nil {
				return fmt.Errorf("%s: broken link %q", rel, target)
			}
		}
		return nil
	})
}

func mustRel(base, target string) string {
	r, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return r
}
