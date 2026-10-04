package driver

import (
	"fmt"

	"kigumi/internal/version"

	"regexp"
	"strconv"
	"strings"
)

// ToolchainVersion is what `Module { kigumi }` is compared against; a module
// written for a newer toolchain is refused instead of misparsed.
const ToolchainVersion = version.Toolchain

// knownHosts are forges where the first three path segments name the repo;
// elsewhere a `.git` segment marks the boundary.
var knownHosts = map[string]bool{"github.com": true, "gitlab.com": true, "bitbucket.org": true, "codeberg.org": true, "sr.ht": true}

// HasHost reports whether a module name starts with a host (a dotted first
// segment), so its repo can be derived.
func HasHost(name string) bool {
	first, _, _ := strings.Cut(name, "/")
	return strings.Contains(first, ".")
}

// CanonicalModuleName lowercases name's host segment (the first, when
// HasHost) and leaves the rest as given; an unqualified name is unchanged.
func CanonicalModuleName(name string) string {
	if !HasHost(name) {
		return name
	}
	host, rest, ok := strings.Cut(name, "/")
	host = strings.ToLower(host)
	if !ok {
		return host
	}
	return host + "/" + rest
}

// RepoOf derives a module name's clone URL and subdirectory; a Require's explicit url wins, and the subdirectory is whatever the name carries beyond it.
func RepoOf(r Require) (url, subdir string, ok bool) {
	if r.URL != "" {
		return r.URL, subdirUnder(r.Name, r.URL), true
	}
	segs := strings.Split(r.Name, "/")
	if !HasHost(r.Name) {
		return "", "", false
	}
	if knownHosts[strings.ToLower(segs[0])] && len(segs) >= 3 {
		return "https://" + strings.Join(segs[:3], "/") + ".git", strings.Join(segs[3:], "/"), true
	}
	for i, s := range segs {
		if strings.HasSuffix(s, ".git") && i > 0 {
			return "https://" + strings.Join(segs[:i+1], "/"), strings.Join(segs[i+1:], "/"), true
		}
	}
	return "", "", false
}

// subdirUnder strips a url's repo part from a module name: the longest tail of the url's path that starts the name segment-wise; no overlap means the repo root.
func subdirUnder(name, url string) string {
	u := url
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.Index(u, "@"); i >= 0 && !strings.Contains(u[:i], "/") {
		u = u[i+1:]
	}
	u = strings.TrimSuffix(strings.Replace(u, ":", "/", 1), "/")
	for _, form := range []string{u, strings.TrimSuffix(u, ".git")} {
		segs := strings.Split(form, "/")
		for i := range segs {
			tail := strings.Join(segs[i:], "/")
			if tail == "" {
				continue
			}
			if name == tail {
				return ""
			}
			if rest, ok := strings.CutPrefix(name, tail+"/"); ok {
				return rest
			}
		}
	}
	return ""
}

// TagOf is the git tag of a version: nested modules tag as
// `<subdir>/<version>`.
func TagOf(subdir, version string) string {
	if subdir == "" {
		return version
	}
	return subdir + "/" + version
}

// commitHash goes up to a sha256 digest's width, not just sha1's 40, so a
// repository using the newer object format still reads as a commit.
var commitHash = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// IsCommitHash tells a commit version from a tag: hex only, 7 to 64 digits.
func IsCommitHash(version string) bool { return commitHash.MatchString(version) }

// TagCommit labels a full commit id with its hash algorithm (40 hex digits
// is sha1, 64 is sha256); a width it does not recognize passes through.
func TagCommit(full string) string {
	switch len(full) {
	case 40:
		return "sha1:" + full
	case 64:
		return "sha256:" + full
	default:
		return full
	}
}

// NormalizeCommit tags a bare-hash commit id from a lock file older than
// TagCommit, so it still compares equal to a freshly resolved one.
func NormalizeCommit(commit string) string {
	if strings.Contains(commit, ":") {
		return commit
	}
	return TagCommit(commit)
}

// ownerOf picks the module a path belongs to: the longest name that is the
// path itself or a segment-wise prefix of it.
func ownerOf(path string, names []string) string {
	best := ""
	for _, n := range names {
		if (path == n || strings.HasPrefix(path, n+"/")) && len(n) > len(best) {
			best = n
		}
	}
	return best
}

// versionAtLeast compares dotted numeric versions; want is a manifest's kigumi field, so a non-numeric component is reported instead of compared as 0.
func versionAtLeast(have, want string) (bool, error) {
	h, w := strings.Split(have, "."), strings.Split(want, ".")
	for i := 0; i < len(h) || i < len(w); i++ {
		var a, b int
		var err error
		if i < len(h) {
			if a, err = strconv.Atoi(h[i]); err != nil {
				return false, fmt.Errorf("version %q: component %q is not a number", have, h[i])
			}
		}
		if i < len(w) {
			if b, err = strconv.Atoi(w[i]); err != nil {
				return false, fmt.Errorf("version %q: component %q is not a number", want, w[i])
			}
		}
		if a != b {
			return a > b, nil
		}
	}
	return true, nil
}
