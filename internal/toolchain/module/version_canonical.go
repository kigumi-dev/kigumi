package module

import (
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// CanonicalVersion is a full SemVer version with no build metadata, the only
// form the revision comparator accepts. Values come from ParseCanonicalVersion
// or ParseVersionTag; an unexported representation keeps unvalidated strings
// out of the comparator.
type CanonicalVersion struct {
	text string
	nums [3]int64
	pre  string
}

// VersionTag is a full SemVer tag before any VCS evidence is applied. Build
// metadata stays separate: dropping it is not canonicalization, because two
// tags that differ only in build metadata may name different commits.
type VersionTag struct {
	Version CanonicalVersion
	Build   string
}

// ParseCanonicalVersion parses a full `v<major>.<minor>.<patch>[-prerelease]`
// SemVer version. Build metadata is rejected here; use ParseVersionTag for a
// tag that carries it. Abbreviations, leading-zero numeric prerelease
// identifiers, and components that do not fit int64 are rejected, so the
// syntax stays exactly Kigumi's and never expands to x/mod's shorthands.
func ParseCanonicalVersion(v string) (CanonicalVersion, error) {
	tag, err := ParseVersionTag(v)
	if err != nil {
		return CanonicalVersion{}, err
	}
	if tag.Build != "" {
		return CanonicalVersion{}, newRevisionError(ErrRevisionSyntax, "version", v,
			"build metadata %q is not part of a canonical version", tag.Build)
	}
	return tag.Version, nil
}

// ParseVersionTag parses Kigumi's full SemVer tag syntax and splits off build
// metadata. The returned Version is the canonical version used for ordering;
// Build retains the metadata for the caller that must resolve the tag against
// a commit.
func ParseVersionTag(v string) (VersionTag, error) {
	m := semverTag.FindStringSubmatch(v)
	if m == nil {
		return VersionTag{}, newRevisionError(ErrRevisionSyntax, "version", v,
			"is not `v<major>.<minor>.<patch>[-prerelease][+build]`")
	}
	var out CanonicalVersion
	for i := range 3 {
		n, err := strconv.ParseInt(m[i+1], 10, 64)
		if err != nil {
			return VersionTag{}, newRevisionError(ErrRevisionOverflow, "version", v,
				"component %q does not fit in int64", m[i+1])
		}
		out.nums[i] = n
	}
	if m[4] != "" {
		out.pre = m[4]
		for _, id := range strings.Split(out.pre, ".") {
			if numeric(id) && len(id) > 1 && id[0] == '0' {
				return VersionTag{}, newRevisionError(ErrRevisionSyntax, "version", v,
					"prerelease identifier %q has a leading zero", id)
			}
		}
	}
	out.text = v
	build := ""
	if i := strings.IndexByte(v, '+'); i >= 0 {
		out.text, build = v[:i], v[i+1:]
	}
	return VersionTag{Version: out, Build: build}, nil
}

// String returns the tag, restoring build metadata when it has any.
func (t VersionTag) String() string {
	if t.Build == "" {
		return t.Version.String()
	}
	return t.Version.String() + "+" + t.Build
}

// String returns the canonical SemVer text, or "" for the zero value.
func (v CanonicalVersion) String() string { return v.text }

// IsZero reports whether v was never built by a parser.
func (v CanonicalVersion) IsZero() bool { return v.text == "" }

// Major returns the major component.
func (v CanonicalVersion) Major() int64 { return v.nums[0] }

// Equal reports whether two canonical versions have the same precedence.
func (v CanonicalVersion) Equal(o CanonicalVersion) bool { return v.Compare(o) == 0 }

// Compare orders two canonical versions per SemVer 2.0. A zero value sorts
// before every parsed version. Malformed text cannot reach it: the only
// constructor validates, and the fields are unexported.
func (v CanonicalVersion) Compare(o CanonicalVersion) int {
	switch {
	case v.text == "" && o.text == "":
		return 0
	case v.text == "":
		return -1
	case o.text == "":
		return 1
	}
	return semver.Compare(v.text, o.text)
}

// withoutBuild drops a build-metadata suffix. It is only used where the
// caller separately validates the tag or holds the commit as evidence.
func withoutBuild(v string) string {
	if i := strings.IndexByte(v, '+'); i >= 0 {
		return v[:i]
	}
	return v
}
