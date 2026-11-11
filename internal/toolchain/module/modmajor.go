package module

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// A module released at v2 or later names its major version in its path
// (`example.com/mod/v2`), as Go does; v0 and v1 modules carry no suffix.
// The suffix is part of the module's identity, so `example.com/mod` and
// `example.com/mod/v2` are two modules that may be required at once.

// majorSuffix matches the last path segment of a major-suffixed name.
var majorSuffix = regexp.MustCompile(`^v(0|[1-9][0-9]*)$`)

// versionSegment matches a segment that looks like a version but cannot be a
// major version suffix (`v1.2.0`, `v1`, `v01`); such a path is a mistake.
var versionSegment = regexp.MustCompile(`^v[0-9]+(?:\.[0-9]+)*(-.*)?$`)

// PathMajor returns name's major version suffix (`/v2`, …), or "" when its
// last segment is not one. A last segment that is a version but not a valid
// major suffix (v0, v1, v01, v2.0) is an error rather than a plain segment.
func PathMajor(name string) (string, error) {
	i := strings.LastIndexByte(name, '/')
	if i < 0 {
		return "", nil
	}
	last := name[i+1:]
	if !majorSuffix.MatchString(last) {
		if versionSegment.MatchString(last) {
			return "", fmt.Errorf("module path %q: %q is not a major version suffix; use /v2 or higher, and leave it off for v0 and v1", name, last)
		}
		return "", nil
	}
	major, err := strconv.ParseInt(last[1:], 10, 64)
	if err != nil {
		return "", fmt.Errorf("module path %q: major version %q does not fit", name, last[1:])
	}
	if major < 2 {
		return "", fmt.Errorf("module path %q: a v%d module has no major suffix; use %q", name, major, name[:i])
	}
	return "/" + last, nil
}

// splitPathMajor returns name's prefix and suffix; ok is false when name has
// no major suffix or when it is malformed (callers that must distinguish the
// two use PathMajor).
func splitPathMajor(name string) (prefix, major string, ok bool) {
	major, err := PathMajor(name)
	if err != nil || major == "" {
		return name, "", false
	}
	return name[:strings.LastIndexByte(name, '/')], major, true
}

// CheckVersionForName checks a version against the module path's major
// suffix: a v2+ tag belongs to a `/vN` path, and a `/vN` path only takes vN
// tags. A commit hash carries no major, so it is accepted either way.
func CheckVersionForName(name, version string) error {
	major, err := PathMajor(name)
	if err != nil {
		return err
	}
	if IsCommitHash(version) {
		return nil
	}
	sv, err := parseSemVer(version)
	if err != nil {
		return err
	}
	if major == "" {
		if sv.nums[0] > 1 {
			return fmt.Errorf("module %s has no major suffix but %s is v%d; the module path must be %s/v%d", name, version, sv.nums[0], name, sv.nums[0])
		}
		return nil
	}
	want, _ := strconv.ParseInt(major[2:], 10, 64)
	if sv.nums[0] != want {
		return fmt.Errorf("module path %s has major suffix %s but version %s is v%d; give a v%d tag or fix the path", name, major, version, sv.nums[0], want)
	}
	return nil
}

// withMajor appends a major version suffix to the module's repository
// subdirectory. A module at the repository root keeps an empty subdirectory
// (its tags are plain `vN.x.y`); a nested module lives in `<subdir>/vN`.
func withMajor(subdir, major string) string {
	if major == "" || subdir == "" {
		return subdir
	}
	return subdir + major
}
