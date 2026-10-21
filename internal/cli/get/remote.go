package get

import (
	"net/url"
	"regexp"
	"strings"

	"kigumi/internal/driver"
	"kigumi/internal/errors"
)

var allowedSchemes = map[string]bool{"https": true, "ssh": true, "git": true, "file": true}

var scpLike = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9.-]*:[^:]`)

func checkRemote(raw string) error {
	if raw == "" || strings.HasPrefix(raw, "-") {
		return errors.NewErrf("refusing repository url %q: it is empty or starts with `-`", raw)
	}
	if strings.ContainsAny(raw, "\x00\n\r\t") {
		return errors.NewErrf("refusing repository url %q: it contains a control character", raw)
	}
	if scpLike.MatchString(raw) {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || !allowedSchemes[u.Scheme] || !strings.Contains(raw, "://") {
		return errors.NewErrf("refusing repository url %q: use an https, ssh, git or file url (or user@host:path)", raw)
	}
	if strings.HasPrefix(u.Host, "-") || strings.HasPrefix(u.User.Username(), "-") || (u.Scheme != "file" && u.Host == "") {
		return errors.NewErrf("refusing repository url %q: bad host", raw)
	}
	return nil
}

func checkRequire(r driver.Require, url string) error {
	if err := checkRemote(url); err != nil {
		return errors.WrapErrf(err, "dependency %s %s", r.Name, r.Version)
	}
	if strings.HasPrefix(r.Version, "-") {
		return errors.NewErrf("dependency %s: refusing version %q: it starts with `-`", r.Name, r.Version)
	}
	return nil
}
