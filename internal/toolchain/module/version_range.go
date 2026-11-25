package module

import (
	"fmt"
	"strconv"
	"strings"
)

// VersionRange is a parsed semver constraint. It supports the forms package
// managers commonly use: `^`, `~`, comparison operators, exact versions,
// wildcards/partial versions, whitespace or comma separated intersections,
// and `||` unions.
type VersionRange struct {
	raw      string
	alts     []rangeAlt
	allowPre bool
}

type rangeAlt []rangeConstraint

type rangeConstraint struct {
	op      string
	version CanonicalVersion
}

// ParseVersionRequest distinguishes an exact revision request from a range.
// Commit hashes and full canonical versions are exact; anything else is a
// range. A nil range means exact.
func ParseVersionRequest(v string) (*VersionRange, error) {
	if v == "" {
		return nil, fmt.Errorf("version is empty")
	}
	if IsCommitHash(v) {
		return nil, nil
	}
	if _, err := ParseCanonicalVersion(v); err == nil {
		return nil, nil
	}
	if _, err := ParseVersionTag(v); err == nil {
		return nil, nil
	}
	return ParseVersionRange(v)
}

// ValidateVersionRequest accepts an exact revision or a semver range.
func ValidateVersionRequest(v string) error {
	_, err := ParseVersionRequest(v)
	return err
}

// ParseVersionRange parses a semver range. The grammar is deliberately
// stricter than npm's: every version still uses Kigumi's full v-prefixed
// SemVer syntax, while partial versions and x/* wildcards are accepted only
// as range operands.
func ParseVersionRange(raw string) (*VersionRange, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, fmt.Errorf("version range is empty")
	}
	r := &VersionRange{raw: text}
	for _, part := range strings.Split(text, "||") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("version range %q has an empty alternative", raw)
		}
		alt, allowPre, err := parseRangeAlt(part)
		if err != nil {
			return nil, fmt.Errorf("version range %q: %w", raw, err)
		}
		r.alts = append(r.alts, alt)
		r.allowPre = r.allowPre || allowPre
	}
	return r, nil
}

func parseRangeAlt(part string) (rangeAlt, bool, error) {
	fields := strings.FieldsFunc(part, func(c rune) bool {
		return c == ' ' || c == '\t' || c == ','
	})
	if len(fields) == 0 {
		return nil, false, fmt.Errorf("empty alternative")
	}
	var alt rangeAlt
	allowPre := false
	for i := range fields {
		tok := fields[i]
		if isRangeOperator(tok) && i+1 < len(fields) {
			tok += fields[i+1]
			i++
		}
		cs, pre, err := parseRangeToken(tok)
		if err != nil {
			return nil, false, err
		}
		alt = append(alt, cs...)
		allowPre = allowPre || pre
	}
	return alt, allowPre, nil
}

func isRangeOperator(s string) bool {
	switch s {
	case "^", "~", ">=", "<=", ">", "<", "=":
		return true
	}
	return false
}

func parseRangeToken(tok string) ([]rangeConstraint, bool, error) {
	op := ""
	for _, candidate := range []string{">=", "<=", ">", "<", "=", "^", "~"} {
		if strings.HasPrefix(tok, candidate) {
			op, tok = candidate, strings.TrimSpace(strings.TrimPrefix(tok, candidate))
			break
		}
	}
	if tok == "*" || tok == "x" || tok == "X" {
		if op != "" && op != "=" {
			return nil, false, fmt.Errorf("wildcard cannot follow %s", op)
		}
		return []rangeConstraint{{op: "any"}}, false, nil
	}
	v, precision, pre, err := parseRangeVersion(tok)
	if err != nil {
		return nil, false, err
	}
	allowPre := pre
	switch op {
	case "":
		if precision == 3 {
			return []rangeConstraint{{op: "=", version: v}}, allowPre, nil
		}
		return partialBounds(v, precision), allowPre, nil
	case "^":
		return caretBounds(v, precision), allowPre, nil
	case "~":
		return tildeBounds(v, precision), allowPre, nil
	case ">=", ">", "<=", "<", "=":
		return []rangeConstraint{{op: op, version: v}}, allowPre, nil
	default:
		return nil, false, fmt.Errorf("unknown range operator %q", op)
	}
}

// parseRangeVersion accepts a full canonical version or a partial version
// with 1-2 numeric components or an x/* wildcard in a trailing component.
func parseRangeVersion(s string) (CanonicalVersion, int, bool, error) {
	for _, candidate := range []string{s, "v" + s} {
		if v, err := ParseCanonicalVersion(candidate); err == nil {
			return v, 3, v.pre != "", nil
		}
		if tag, err := ParseVersionTag(candidate); err == nil {
			return tag.Version, 3, tag.Version.pre != "", nil
		}
	}
	if strings.Count(s, ".") == 2 && !strings.ContainsAny(s, "xX*") {
		for _, candidate := range []string{s, "v" + s} {
			if _, err := ParseCanonicalVersion(candidate); err != nil {
				return CanonicalVersion{}, 0, false, err
			}
		}
	}
	body := strings.TrimPrefix(s, "v")
	core, hasPre := body, false
	if i := strings.IndexAny(core, "-+"); i >= 0 {
		hasPre = core[i] == '-'
		core = core[:i]
	}
	if hasPre {
		return CanonicalVersion{}, 0, false, fmt.Errorf("%q: prerelease needs a full three-component version", s)
	}
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return CanonicalVersion{}, 0, false, fmt.Errorf("%q is not a semver range operand", s)
	}
	nums := [3]int64{}
	precision := 0
	wildcard := false
	for i, p := range parts {
		if p == "x" || p == "X" || p == "*" {
			if i == 0 {
				return CanonicalVersion{}, 0, false, fmt.Errorf("%q has a wildcard major", s)
			}
			wildcard = true
			continue
		}
		if wildcard {
			return CanonicalVersion{}, 0, false, fmt.Errorf("%q has a component after a wildcard", s)
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n < 0 {
			return CanonicalVersion{}, 0, false, fmt.Errorf("%q: %q is not a version component", s, p)
		}
		nums[i] = n
		precision = i + 1
	}
	if precision == 0 {
		return CanonicalVersion{}, 0, false, fmt.Errorf("%q has no concrete version component", s)
	}
	v, err := canonicalFromNums(nums)
	if err != nil {
		return CanonicalVersion{}, 0, false, err
	}
	return v, precision, false, nil
}

func canonicalFromNums(nums [3]int64) (CanonicalVersion, error) {
	return ParseCanonicalVersion(fmt.Sprintf("v%d.%d.%d", nums[0], nums[1], nums[2]))
}

func partialBounds(v CanonicalVersion, precision int) []rangeConstraint {
	lower := v
	var upper CanonicalVersion
	switch precision {
	case 1:
		upper, _ = canonicalFromNums([3]int64{v.nums[0] + 1, 0, 0})
	case 2:
		upper, _ = canonicalFromNums([3]int64{v.nums[0], v.nums[1] + 1, 0})
	default:
		return []rangeConstraint{{op: "=", version: v}}
	}
	return []rangeConstraint{{op: ">=", version: lower}, {op: "<", version: upper}}
}

func caretBounds(v CanonicalVersion, precision int) []rangeConstraint {
	lower := v
	var upper CanonicalVersion
	switch {
	case v.nums[0] > 0 || precision == 1:
		upper, _ = canonicalFromNums([3]int64{v.nums[0] + 1, 0, 0})
	case v.nums[1] > 0 || precision == 2:
		upper, _ = canonicalFromNums([3]int64{0, v.nums[1] + 1, 0})
	default:
		upper, _ = canonicalFromNums([3]int64{0, 0, v.nums[2] + 1})
	}
	return []rangeConstraint{{op: ">=", version: lower}, {op: "<", version: upper}}
}

func tildeBounds(v CanonicalVersion, precision int) []rangeConstraint {
	lower := v
	var upper CanonicalVersion
	if precision == 1 {
		upper, _ = canonicalFromNums([3]int64{v.nums[0] + 1, 0, 0})
	} else {
		upper, _ = canonicalFromNums([3]int64{v.nums[0], v.nums[1] + 1, 0})
	}
	return []rangeConstraint{{op: ">=", version: lower}, {op: "<", version: upper}}
}

// String returns the range text as written.
func (r *VersionRange) String() string {
	if r == nil {
		return ""
	}
	return r.raw
}

// IsAny reports whether the range accepts every version.
func (r *VersionRange) IsAny() bool {
	return r == nil || len(r.alts) == 1 && len(r.alts[0]) == 1 && r.alts[0][0].op == "any"
}

// Accepts reports whether v satisfies the range. Prereleases are accepted
// only when the range mentions a prerelease, matching semver tooling.
func (r *VersionRange) Accepts(v CanonicalVersion) bool {
	if r == nil {
		return false
	}
	if v.IsZero() || v.pre != "" && !r.allowPre {
		return false
	}
	for _, alt := range r.alts {
		ok := true
		for _, c := range alt {
			if !c.test(v) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// AcceptsString parses a tag and applies Accepts. Build metadata is ignored
// for range comparison, as semver requires.
func (r *VersionRange) AcceptsString(s string) bool {
	tag, err := ParseVersionTag(s)
	if err != nil {
		return false
	}
	return r.Accepts(tag.Version)
}

func (c rangeConstraint) test(v CanonicalVersion) bool {
	switch c.op {
	case "any":
		return true
	case "=":
		return v.Compare(c.version) == 0
	case ">":
		return v.Compare(c.version) > 0
	case ">=":
		return v.Compare(c.version) >= 0
	case "<":
		return v.Compare(c.version) < 0
	case "<=":
		return v.Compare(c.version) <= 0
	}
	return false
}
