package module

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"kigumi/internal/lang/diag"
)

// RevisionErrorCode is a stable machine-readable reason for rejecting a
// revision request or resolved revision.
type RevisionErrorCode string

const (
	ErrRevisionSyntax        RevisionErrorCode = "revision.syntax"
	ErrRevisionHash          RevisionErrorCode = "revision.hash"
	ErrRevisionPseudoTime    RevisionErrorCode = "revision.pseudo-time"
	ErrRevisionPseudoBase    RevisionErrorCode = "revision.pseudo-base"
	ErrRevisionMajorMismatch RevisionErrorCode = "revision.major-mismatch"
	ErrRevisionSource        RevisionErrorCode = "revision.source"
	ErrRevisionAmbiguous     RevisionErrorCode = "revision.ambiguous"
	ErrRevisionOverflow      RevisionErrorCode = "revision.overflow"
)

func (c RevisionErrorCode) String() string { return string(c) }

// RevisionError explains why a revision value is invalid and where it was
// declared. Loc is zero when the caller has no source position.
type RevisionError struct {
	Code  RevisionErrorCode
	Field string
	Value string
	Loc   diag.Location
	Err   error
}

func newRevisionError(code RevisionErrorCode, field, value, format string, args ...any) *RevisionError {
	return &RevisionError{Code: code, Field: field, Value: value, Err: fmt.Errorf(format, args...)}
}

// WithLocation attaches the declaration site without changing the reason.
func (e *RevisionError) WithLocation(loc diag.Location) *RevisionError {
	e.Loc = loc
	return e
}

func (e *RevisionError) Error() string {
	var b strings.Builder
	if e.Field != "" {
		b.WriteString(e.Field)
		if e.Value != "" {
			b.WriteByte(' ')
			b.WriteString(fmt.Sprintf("%q", e.Value))
		}
		b.WriteString(": ")
	}
	if e.Err != nil {
		b.WriteString(e.Err.Error())
	} else {
		b.WriteString(e.Code.String())
	}
	if e.Loc.Span.Start != 0 {
		fmt.Fprintf(&b, " (at byte %d)", e.Loc.Span.Start)
	}
	return b.String()
}

func (e *RevisionError) Unwrap() error { return e.Err }

// Diagnostic renders the error at its declaration site and carries the typed
// code for machine-readable reports.
func (e *RevisionError) Diagnostic() diag.Diagnostic {
	d := diag.Errorf(e.Loc, e.Error())
	d.Code = e.Code.String()
	return d
}

// CommitAlgorithm names the hash algorithm of a full commit id.
type CommitAlgorithm uint8

const (
	CommitSHA1 CommitAlgorithm = iota
	CommitSHA256
)

func (a CommitAlgorithm) String() string {
	switch a {
	case CommitSHA1:
		return "sha1"
	case CommitSHA256:
		return "sha256"
	default:
		return fmt.Sprintf("commit-algorithm(%d)", uint8(a))
	}
}

// CommitAlgorithmForLen returns the algorithm whose full id has n hex digits.
func CommitAlgorithmForLen(n int) (CommitAlgorithm, bool) {
	switch n {
	case 40:
		return CommitSHA1, true
	case 64:
		return CommitSHA256, true
	default:
		return 0, false
	}
}

// ParseCommitAlgorithm parses "sha1" or "sha256".
func ParseCommitAlgorithm(s string) (CommitAlgorithm, error) {
	switch s {
	case "sha1":
		return CommitSHA1, nil
	case "sha256":
		return CommitSHA256, nil
	default:
		return 0, newRevisionError(ErrRevisionHash, "commit", s, "unknown hash algorithm; want sha1 or sha256")
	}
}

// CommitID is a full commit id with its hash algorithm.
type CommitID struct {
	Algorithm CommitAlgorithm
	Hex       string
}

// ParseCommitID parses the lock-file spelling `sha1:<40 hex>` or
// `sha256:<64 hex>`.
func ParseCommitID(s string) (CommitID, error) {
	algoText, hex, ok := strings.Cut(s, ":")
	if !ok {
		return CommitID{}, newRevisionError(ErrRevisionHash, "commit", s, "full commit id needs a `sha1:` or `sha256:` prefix")
	}
	algo, err := ParseCommitAlgorithm(algoText)
	if err != nil {
		return CommitID{}, err
	}
	c := CommitID{Algorithm: algo, Hex: hex}
	if err := c.Validate(); err != nil {
		return CommitID{}, err
	}
	return c, nil
}

// ParseFullCommitHex parses a bare full commit id, inferring the algorithm
// from its width.
func ParseFullCommitHex(s string) (CommitID, error) {
	algo, ok := CommitAlgorithmForLen(len(s))
	if !ok {
		return CommitID{}, newRevisionError(ErrRevisionHash, "commit", s,
			"a full commit id is 40 (sha1) or 64 (sha256) lowercase hex digits; got %d", len(s))
	}
	c := CommitID{Algorithm: algo, Hex: s}
	if err := c.Validate(); err != nil {
		return CommitID{}, err
	}
	return c, nil
}

// Validate checks the algorithm and the exact lowercase hex width.
func (c CommitID) Validate() error {
	want := 0
	switch c.Algorithm {
	case CommitSHA1:
		want = 40
	case CommitSHA256:
		want = 64
	default:
		return newRevisionError(ErrRevisionHash, "commit", c.String(), "unknown hash algorithm")
	}
	if len(c.Hex) != want {
		return newRevisionError(ErrRevisionHash, "commit", c.String(),
			"%s ids are %d hex digits; got %d", c.Algorithm, want, len(c.Hex))
	}
	for _, r := range c.Hex {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return newRevisionError(ErrRevisionHash, "commit", c.String(),
				"commit ids are lowercase hex only")
		}
	}
	return nil
}

// String spells the id as stored in a lock.
func (c CommitID) String() string {
	if c.Algorithm == CommitSHA1 && c.Hex == "" {
		return ""
	}
	return c.Algorithm.String() + ":" + c.Hex
}

// Short returns the 12-hex-digit revision suffix used by pseudo-versions.
func (c CommitID) Short() string {
	if len(c.Hex) <= 12 {
		return c.Hex
	}
	return c.Hex[:12]
}

// MatchCommitPrefix resolves a 7-64 digit lowercase hex prefix against full
// commit ids. A prefix matching more than one distinct commit is ambiguous;
// it is never silently widened or narrowed.
func MatchCommitPrefix(prefix string, candidates []CommitID) (CommitID, error) {
	if !IsCommitHash(prefix) {
		return CommitID{}, newRevisionError(ErrRevisionHash, "commit", prefix,
			"a commit prefix is 7 to 64 lowercase hex digits")
	}
	seen := map[string]CommitID{}
	var matches []string
	for _, c := range candidates {
		if err := c.Validate(); err != nil {
			return CommitID{}, err
		}
		if !strings.HasPrefix(c.Hex, prefix) {
			continue
		}
		key := c.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = c
		matches = append(matches, key)
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return CommitID{}, newRevisionError(ErrRevisionHash, "commit", prefix, "no commit starts with this prefix")
	case 1:
		return seen[matches[0]], nil
	default:
		return CommitID{}, newRevisionError(ErrRevisionAmbiguous, "commit", prefix,
			"matches %d commits: %s", len(matches), strings.Join(matches, ", "))
	}
}

// SourceIdentity is the resolved identity of a module source. Name is the real
// module name, not an alias; URL is the existing URL key (empty for a local
// source); Subdir is the module directory inside the repository.
type SourceIdentity struct {
	Name   string
	URL    string
	Subdir string
}

// NewSourceIdentity validates and builds a source identity.
func NewSourceIdentity(name, url, subdir string) (SourceIdentity, error) {
	s := SourceIdentity{Name: name, URL: url, Subdir: subdir}
	if err := s.Validate(); err != nil {
		return SourceIdentity{}, err
	}
	return s, nil
}

// Validate checks the module name, its path-major suffix, the URL key, and
// that the subdirectory cannot escape the repository.
func (s SourceIdentity) Validate() error {
	if err := ValidModuleName(s.Name); err != nil {
		return newRevisionError(ErrRevisionSource, "module", s.Name, "%v", err)
	}
	if s.URL != "" {
		if strings.TrimSpace(s.URL) != s.URL {
			return newRevisionError(ErrRevisionSource, "source url", s.URL, "has leading or trailing space")
		}
		for _, r := range s.URL {
			if r <= ' ' || r == 0x7f {
				return newRevisionError(ErrRevisionSource, "source url", s.URL, "contains control or whitespace characters")
			}
		}
	}
	if s.Subdir != "" {
		if strings.HasPrefix(s.Subdir, "/") || strings.Contains(s.Subdir, `\`) {
			return newRevisionError(ErrRevisionSource, "source subdir", s.Subdir, "must be a relative slash path")
		}
		if clean := path.Clean(s.Subdir); clean != s.Subdir || clean == "." {
			return newRevisionError(ErrRevisionSource, "source subdir", s.Subdir, "must be a clean relative path")
		}
		for _, seg := range strings.Split(s.Subdir, "/") {
			if seg == "" || seg == "." || seg == ".." {
				return newRevisionError(ErrRevisionSource, "source subdir", s.Subdir, "escapes the repository")
			}
		}
	}
	return nil
}

// RevisionRequest is one raw version request before VCS evidence is applied.
// Version is exactly what the manifest declared: a tag, pseudo-version, or
// commit hash. Alias is the effective key when the Require is aliased.
type RevisionRequest struct {
	Version string
	Source  SourceIdentity
	Alias   string
	Loc     diag.Location
}

// NewRevisionRequest converts an existing Require without changing resolver
// behaviour. The source URL key and subdirectory come from the same RepoOf
// rules the resolver uses today.
func NewRevisionRequest(r Require) (RevisionRequest, error) {
	_, subdir, _ := RepoOf(r)
	req := RevisionRequest{
		Version: r.Version,
		Source:  SourceIdentity{Name: r.Name, URL: RepoURLKey(r), Subdir: subdir},
		Alias:   r.Alias,
	}
	if err := req.Validate(); err != nil {
		return RevisionRequest{}, err
	}
	return req, nil
}

// Validate checks the source identity, the request syntax, the path major,
// and the alias grammar. It never resolves or compares the version.
func (r RevisionRequest) Validate() error {
	if err := r.Source.Validate(); err != nil {
		return err
	}
	parsed, err := ParseRequestVersion(r.Version)
	if err != nil {
		if re, ok := err.(*RevisionError); ok {
			return re.WithLocation(r.Loc)
		}
		return newRevisionError(ErrRevisionSyntax, "version", r.Version, "%v", err).WithLocation(r.Loc)
	}
	if parsed.Kind != RequestVersionCommit {
		if err := CheckVersionForName(r.Source.Name, r.Version); err != nil {
			return newRevisionError(ErrRevisionMajorMismatch, "version", r.Version, "%v", err).WithLocation(r.Loc)
		}
	}
	if r.Alias != "" {
		if err := ValidAlias(r.Alias); err != nil {
			return newRevisionError(ErrRevisionSource, "alias", r.Alias, "%v", err).WithLocation(r.Loc)
		}
	}
	return nil
}

// RevisionOrigin records which evidence produced a resolved revision.
type RevisionOrigin uint8

const (
	RevisionTag RevisionOrigin = iota
	RevisionPseudo
	RevisionCommit
	RevisionLocal
)

func (o RevisionOrigin) String() string {
	switch o {
	case RevisionTag:
		return "tag"
	case RevisionPseudo:
		return "pseudo"
	case RevisionCommit:
		return "commit"
	case RevisionLocal:
		return "local"
	default:
		return fmt.Sprintf("revision-origin(%d)", uint8(o))
	}
}

// RevisionBase is the base tag evidence of a pseudo-version.
type RevisionBase struct {
	Version CanonicalVersion
	Tag     string
	Commit  CommitID
}

// Validate checks that the base has a canonical version, a full tag, and a
// full commit.
func (b RevisionBase) Validate() error {
	if b.Version.IsZero() {
		return newRevisionError(ErrRevisionPseudoBase, "base version", "", "is missing")
	}
	if b.Tag == "" {
		return newRevisionError(ErrRevisionPseudoBase, "base tag", "", "is missing")
	}
	tag, err := ParseRequestVersion(b.Tag)
	if err != nil {
		return newRevisionError(ErrRevisionPseudoBase, "base tag", b.Tag, "%v", err)
	}
	if tag.Kind != RequestVersionTag {
		return newRevisionError(ErrRevisionPseudoBase, "base tag", b.Tag, "must be a tag, not a %s", tag.Kind)
	}
	if !tag.Tag.Version.Equal(b.Version) {
		return newRevisionError(ErrRevisionPseudoBase, "base tag", b.Tag,
			"names %s but the base version is %s", tag.Tag.Version.String(), b.Version.String())
	}
	if err := b.Commit.Validate(); err != nil {
		return err
	}
	return nil
}

// ResolvedRevision is a request plus the canonical version and full commit it
// resolved to. Local sources are typed by Origin/Local and never participate
// in remote version ordering.
type ResolvedRevision struct {
	Request RevisionRequest
	Version CanonicalVersion
	Commit  CommitID
	Time    time.Time
	Origin  RevisionOrigin
	Base    *RevisionBase
	Local   bool
}

// Validate checks the canonical version, source, commit, timestamp, origin,
// and pseudo-version evidence for internal consistency.
func (r ResolvedRevision) Validate() error {
	if err := r.Request.Validate(); err != nil {
		return err
	}
	if r.Version.IsZero() {
		return newRevisionError(ErrRevisionSyntax, "version", "", "resolved revision has no canonical version")
	}
	if err := CheckVersionForName(r.Request.Source.Name, r.Version.String()); err != nil {
		return newRevisionError(ErrRevisionMajorMismatch, "version", r.Version.String(), "%v", err)
	}
	if r.Local != (r.Origin == RevisionLocal) {
		return newRevisionError(ErrRevisionSource, "source", r.Request.Source.Name,
			"Local and RevisionLocal must be set together")
	}
	if r.Origin == RevisionLocal {
		if r.Commit.String() != "" {
			if err := r.Commit.Validate(); err != nil {
				return err
			}
		}
		if r.Base != nil {
			return newRevisionError(ErrRevisionPseudoBase, "base", "", "a local source has no remote base")
		}
		return nil
	}
	if err := r.Commit.Validate(); err != nil {
		return err
	}
	if r.Time.IsZero() {
		return newRevisionError(ErrRevisionPseudoTime, "time", "", "resolved remote revision has no committer time")
	}
	if r.Time.Nanosecond() != 0 {
		return newRevisionError(ErrRevisionPseudoTime, "time", r.Time.Format(time.RFC3339Nano),
			"committer time must be whole UTC seconds")
	}
	if _, offset := r.Time.Zone(); offset != 0 {
		return newRevisionError(ErrRevisionPseudoTime, "time", r.Time.Format(time.RFC3339Nano),
			"committer time must be UTC")
	}
	switch r.Origin {
	case RevisionTag:
		if r.Base != nil {
			return newRevisionError(ErrRevisionPseudoBase, "base", "", "a tag-resolved revision carries no base evidence")
		}
	case RevisionPseudo:
		p, err := ParsePseudoVersion(r.Version.String())
		if err != nil {
			return err
		}
		if !p.Version.Equal(r.Version) {
			return newRevisionError(ErrRevisionPseudoBase, "version", r.Version.String(),
				"parsed pseudo-version is %s", p.Version.String())
		}
		if err := ValidatePseudoVersion(r.Request.Source.Name, p, r.Commit, r.Base); err != nil {
			return err
		}
	case RevisionCommit:
		if r.Base != nil {
			return newRevisionError(ErrRevisionPseudoBase, "base", "", "a commit-resolved revision carries no base evidence")
		}
	default:
		return newRevisionError(ErrRevisionSource, "origin", r.Origin.String(), "unknown revision origin")
	}
	return nil
}
