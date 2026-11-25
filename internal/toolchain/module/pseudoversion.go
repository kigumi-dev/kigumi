package module

import (
	"fmt"
	"math"
	"strconv"
	"time"

	xmod "golang.org/x/mod/module"
)

// A pseudo-version names an untagged commit: vX.0.0-T-R with no base, or
// vX.Y.(Z+1)-0.T-R / vX.Y.Z-pre.0.T-R with one. T is the commit's committer
// time in UTC to the second and R is the first 12 hex digits of its full id.
// The forms and ordering come from Go's pseudo-version specification; Kigumi
// keeps its stricter tag syntax and lowercase-hex full commit ids.

const (
	pseudoTimeLayout = "20060102150405"
	pseudoRevLen     = 12
)

// PseudoVersion is a parsed pseudo-version. Version is the canonical form
// without build metadata; Build retains optional metadata such as
// `+incompatible` but never participates in ordering.
type PseudoVersion struct {
	Version CanonicalVersion
	Build   string
	Time    time.Time
	Rev     string
	Base    *CanonicalVersion
}

// String returns the pseudo-version, restoring build metadata when present.
func (p PseudoVersion) String() string {
	if p.Build == "" {
		return p.Version.String()
	}
	return p.Version.String() + "+" + p.Build
}

// ParsePseudoVersion parses and internally validates a pseudo-version: the
// timestamp must be a real UTC second, the revision suffix exactly 12
// lowercase hex digits, and the base relationship must regenerate the exact
// canonical text.
func ParsePseudoVersion(v string) (PseudoVersion, error) {
	if !xmod.IsPseudoVersion(v) {
		return PseudoVersion{}, newRevisionError(ErrRevisionSyntax, "version", v, "is not a pseudo-version")
	}
	tag, err := ParseVersionTag(v)
	if err != nil {
		return PseudoVersion{}, err
	}
	t, err := xmod.PseudoVersionTime(v)
	if err != nil {
		return PseudoVersion{}, newRevisionError(ErrRevisionPseudoTime, "version", v, "%v", err)
	}
	if t.Year() < 1 || t.Year() > 9999 {
		return PseudoVersion{}, newRevisionError(ErrRevisionPseudoTime, "version", v,
			"timestamp year %d is outside 1-9999", t.Year())
	}
	rev, err := xmod.PseudoVersionRev(v)
	if err != nil {
		return PseudoVersion{}, newRevisionError(ErrRevisionSyntax, "version", v, "%v", err)
	}
	if !validPseudoRev(rev) {
		return PseudoVersion{}, newRevisionError(ErrRevisionHash, "version", v,
			"revision suffix must be exactly %d lowercase hex digits", pseudoRevLen)
	}
	baseText, err := xmod.PseudoVersionBase(v)
	if err != nil {
		return PseudoVersion{}, newRevisionError(ErrRevisionPseudoBase, "version", v, "%v", err)
	}
	baseText = withoutBuild(baseText)
	var base *CanonicalVersion
	if baseText != "" {
		b, err := ParseCanonicalVersion(baseText)
		if err != nil {
			return PseudoVersion{}, newRevisionError(ErrRevisionPseudoBase, "version", v,
				"base %q is not canonical: %v", baseText, err)
		}
		base = &b
	}
	p := PseudoVersion{Version: tag.Version, Build: tag.Build, Time: t, Rev: rev, Base: base}
	want, err := pseudoText(p.Version.nums[0], p.Base, p.Time, p.Rev)
	if err != nil {
		return PseudoVersion{}, newRevisionError(ErrRevisionPseudoBase, "version", v, "%v", err)
	}
	if want != tag.Version.String() {
		return PseudoVersion{}, newRevisionError(ErrRevisionPseudoBase, "version", v,
			"base evidence does not produce this pseudo-version (expected %s)", want)
	}
	return p, nil
}

// NewPseudoVersion generates the pseudo-version for a commit with the given
// committer time and full id. A nil base yields vM.0.0-T-R, where M is the
// module path's major (`0` for an unsuffixed path); a release base yields
// vX.Y.(Z+1)-0.T-R; a prerelease base yields vX.Y.Z-pre.0.T-R.
func NewPseudoVersion(name string, base *CanonicalVersion, t time.Time, rev CommitID) (CanonicalVersion, error) {
	if err := ValidModuleName(name); err != nil {
		return CanonicalVersion{}, newRevisionError(ErrRevisionSource, "module", name, "%v", err)
	}
	if err := rev.Validate(); err != nil {
		return CanonicalVersion{}, err
	}
	major, err := majorForName(name)
	if err != nil {
		return CanonicalVersion{}, err
	}
	if base != nil {
		if err := CheckVersionForName(name, base.String()); err != nil {
			return CanonicalVersion{}, newRevisionError(ErrRevisionMajorMismatch, "version", base.String(), "%v", err)
		}
		major = base.nums[0]
	}
	text, err := pseudoText(major, base, t, rev.Short())
	if err != nil {
		return CanonicalVersion{}, err
	}
	v, err := ParseCanonicalVersion(text)
	if err != nil {
		return CanonicalVersion{}, err
	}
	return v, nil
}

// ValidatePseudoVersion checks a parsed pseudo-version against the resolved
// full commit and base evidence. A pseudo-version with a base requires the
// three-part base evidence; the no-base form requires none.
func ValidatePseudoVersion(name string, p PseudoVersion, commit CommitID, base *RevisionBase) error {
	if err := ValidModuleName(name); err != nil {
		return newRevisionError(ErrRevisionSource, "module", name, "%v", err)
	}
	if err := CheckVersionForName(name, p.Version.String()); err != nil {
		return newRevisionError(ErrRevisionMajorMismatch, "version", p.Version.String(), "%v", err)
	}
	if err := commit.Validate(); err != nil {
		return err
	}
	if p.Rev != commit.Short() {
		return newRevisionError(ErrRevisionHash, "version", p.String(),
			"revision suffix %q does not match commit %s (want %q)", p.Rev, commit.String(), commit.Short())
	}
	if p.Base == nil {
		if base != nil {
			return newRevisionError(ErrRevisionPseudoBase, "version", p.String(),
				"has no base but base evidence was supplied")
		}
		return nil
	}
	if base == nil {
		return newRevisionError(ErrRevisionPseudoBase, "version", p.String(),
			"base version %s needs base tag and commit evidence", p.Base.String())
	}
	if err := base.Validate(); err != nil {
		return err
	}
	if !base.Version.Equal(*p.Base) {
		return newRevisionError(ErrRevisionPseudoBase, "version", p.String(),
			"base is %s but evidence says %s", p.Base.String(), base.Version.String())
	}
	return nil
}

// majorForName returns the major a base-less pseudo-version starts from: 0 for
// an unsuffixed path, N for a `/vN` path.
func majorForName(name string) (int64, error) {
	suffix, err := PathMajor(name)
	if err != nil {
		return 0, newRevisionError(ErrRevisionSource, "module", name, "%v", err)
	}
	if suffix == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(suffix[2:], 10, 64)
	if err != nil {
		return 0, newRevisionError(ErrRevisionOverflow, "module", name, "major version %q does not fit", suffix[2:])
	}
	return n, nil
}

// pseudoText is the generation arithmetic. It refuses patch overflow and
// timestamps that cannot be written as a 14-digit UTC second.
func pseudoText(major int64, base *CanonicalVersion, t time.Time, rev string) (string, error) {
	if !validPseudoRev(rev) {
		return "", newRevisionError(ErrRevisionHash, "revision", rev,
			"pseudo-version revision is exactly %d lowercase hex digits", pseudoRevLen)
	}
	timestamp, err := pseudoTimestamp(t)
	if err != nil {
		return "", err
	}
	if base == nil {
		return fmt.Sprintf("v%d.0.0-%s-%s", major, timestamp, rev), nil
	}
	if base.nums[0] != major {
		return "", newRevisionError(ErrRevisionMajorMismatch, "base version", base.String(),
			"major %d does not match pseudo-version major %d", base.nums[0], major)
	}
	if base.pre != "" {
		return base.text + ".0." + timestamp + "-" + rev, nil
	}
	if base.nums[2] == math.MaxInt64 {
		return "", newRevisionError(ErrRevisionOverflow, "base version", base.String(),
			"cannot increment patch %d", base.nums[2])
	}
	return fmt.Sprintf("v%d.%d.%d-0.%s-%s", base.nums[0], base.nums[1], base.nums[2]+1, timestamp, rev), nil
}

// pseudoTimestamp formats a committer time as UTC seconds, rejecting times
// whose year is outside the four-digit range.
func pseudoTimestamp(t time.Time) (string, error) {
	u := t.UTC().Truncate(time.Second)
	if u.Year() < 1 || u.Year() > 9999 {
		return "", newRevisionError(ErrRevisionPseudoTime, "time", t.Format(time.RFC3339Nano),
			"year %d cannot be written as YYYYMMDDhhmmss", u.Year())
	}
	return u.Format(pseudoTimeLayout), nil
}

func validPseudoRev(rev string) bool {
	if len(rev) != pseudoRevLen {
		return false
	}
	for _, r := range rev {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// RequestVersionKind classifies a raw Require.version before resolution.
type RequestVersionKind uint8

const (
	RequestVersionTag RequestVersionKind = iota
	RequestVersionPseudo
	RequestVersionCommit
)

func (k RequestVersionKind) String() string {
	switch k {
	case RequestVersionTag:
		return "tag"
	case RequestVersionPseudo:
		return "pseudo"
	case RequestVersionCommit:
		return "commit"
	default:
		return fmt.Sprintf("request-version-kind(%d)", uint8(k))
	}
}

// ParsedRequestVersion is a syntactically valid request version. Exactly one
// of Tag, Pseudo, and CommitHex is populated according to Kind.
type ParsedRequestVersion struct {
	Raw       string
	Kind      RequestVersionKind
	Tag       VersionTag
	Pseudo    PseudoVersion
	CommitHex string
}

// ParseRequestVersion classifies a raw request version without resolving it.
// Pseudo-version syntax is reserved before tag syntax, so a Git tag that merely
// looks like a pseudo-version can never become resolution evidence.
func ParseRequestVersion(v string) (ParsedRequestVersion, error) {
	out := ParsedRequestVersion{Raw: v}
	switch {
	case IsCommitHash(v):
		out.Kind, out.CommitHex = RequestVersionCommit, v
		return out, nil
	case xmod.IsPseudoVersion(v):
		p, err := ParsePseudoVersion(v)
		if err != nil {
			return ParsedRequestVersion{}, err
		}
		out.Kind, out.Pseudo = RequestVersionPseudo, p
		return out, nil
	default:
		t, err := ParseVersionTag(v)
		if err != nil {
			return ParsedRequestVersion{}, err
		}
		out.Kind, out.Tag = RequestVersionTag, t
		return out, nil
	}
}
