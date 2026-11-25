package module

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// This file holds the format-independent meaning of a lock file. A codec
// turns records into a LockState, and migration steps rewrite one LockState
// into another without changing how a dependency resolves.
//
// The Lock/LockEntry types and ReadLock/WriteLockState APIs in lockfile.go
// expose the selected entries of the same document; the headerless legacy
// lock has no parser here.

// LockKind is the lock file's subject. A script uses LockKindModule, like a
// standalone module.
type LockKind uint8

const (
	LockKindModule LockKind = iota + 1
	LockKindWorkspace
)

// String returns the header record name for kind.
func (k LockKind) String() string {
	switch k {
	case LockKindModule:
		return "ModuleLock"
	case LockKindWorkspace:
		return "WorkspaceLock"
	default:
		return fmt.Sprintf("LockKind(%d)", uint8(k))
	}
}

// Valid reports whether k is a kind this package knows.
func (k LockKind) Valid() bool { return k == LockKindModule || k == LockKindWorkspace }

// ParseLockKind reads a header record name (`ModuleLock` or `WorkspaceLock`).
func ParseLockKind(s string) (LockKind, error) {
	switch s {
	case "ModuleLock":
		return LockKindModule, nil
	case "WorkspaceLock":
		return LockKindWorkspace, nil
	default:
		return 0, fmt.Errorf("unknown lock kind %q", s)
	}
}

// LockSource is one Source record: the identity a dependency is fetched
// from. ID is derived from Identity, never from a cache path or checkout.
type LockSource struct {
	ID       string
	Identity SourceIdentity
}

// LockBase is the recorded evidence for a pseudo-version's base tag. It is the
// canonical revision base type so lock and resolver evidence cannot drift.
type LockBase = RevisionBase

// LockResolution is one Resolution record: what one requested revision
// resolved to, including requests that lost graph selection. Loser
// resolutions are evidence for offline re-selection and must survive a
// format migration.
type LockResolution struct {
	ID        string
	SourceID  string
	Requested string
	Version   CanonicalVersion
	Commit    CommitID
	Time      time.Time
	Manifest  string
	Tag       string
	Base      *LockBase
}

// LockBinding is one selected Lock record, keyed by effective Require key
// (the alias when present, else the real module name).
//
// A complete binding carries SourceID, ResolutionID and a canonical Version.
// A selected-only binding carries none of those and instead records Requested,
// the raw request the resolver selected (a tag, pseudo-version or commit
// hash). The selected-only shape exists because the fetch path has no commit
// time or pseudo-version base evidence, so a Resolution cannot be written
// without inventing fields; it is never a licence to omit a known value.
type LockBinding struct {
	Name         string
	Alias        string
	SourceID     string
	ResolutionID string
	Version      CanonicalVersion
	Requested    string
	Commit       CommitID
	Hash         string
	URL          string
}

// Key is the effective dependency key this binding is selected under.
func (b LockBinding) Key() string {
	if b.Alias != "" {
		return b.Alias
	}
	return b.Name
}

// SelectedVersion is the revision text this binding records for its entry:
// the raw request for a selected-only binding, canonical otherwise.
func (b LockBinding) SelectedVersion() string {
	if b.Requested != "" {
		return b.Requested
	}
	return b.Version.String()
}

// complete reports whether the binding references Source and Resolution
// records. A selected-only binding has neither reference.
func (b LockBinding) complete() bool {
	return b.SourceID != "" || b.ResolutionID != "" || b.Requested == ""
}

// LockEdge is one selected dependency edge. Edges are derived from
// manifests rather than encoded as their own record, so a migration must
// carry them through unchanged when they are present in memory.
type LockEdge struct {
	From, To string
}

// LockState is the meaning of a lock file: selected bindings, every request
// examined while selecting, source identities, integrity values and input
// evidence. Format-specific record names and record IDs are representation,
// not identity; migration compares LockState values.
type LockState struct {
	Kind        LockKind
	Format      int
	Resolver    string
	Inputs      string
	Sources     []LockSource
	Resolutions []LockResolution
	Locks       []LockBinding
	Edges       []LockEdge
}

// Clone deep-copies s so a migration step cannot mutate its input.
func (s LockState) Clone() LockState {
	out := s
	out.Sources = append([]LockSource(nil), s.Sources...)
	out.Resolutions = append([]LockResolution(nil), s.Resolutions...)
	for i := range out.Resolutions {
		if s.Resolutions[i].Base != nil {
			b := *s.Resolutions[i].Base
			out.Resolutions[i].Base = &b
		}
	}
	out.Locks = append([]LockBinding(nil), s.Locks...)
	out.Edges = append([]LockEdge(nil), s.Edges...)
	return out
}

// SortRecords orders records the way EncodeLock writes them, so decoding a
// canonical encoding is already canonical and re-encoding is byte-stable.
func (s *LockState) SortRecords() {
	sort.Slice(s.Sources, func(i, j int) bool { return s.Sources[i].ID < s.Sources[j].ID })
	sort.Slice(s.Resolutions, func(i, j int) bool { return s.Resolutions[i].ID < s.Resolutions[j].ID })
	sort.Slice(s.Locks, func(i, j int) bool { return s.Locks[i].Key() < s.Locks[j].Key() })
	sort.Slice(s.Edges, func(i, j int) bool {
		if s.Edges[i].From != s.Edges[j].From {
			return s.Edges[i].From < s.Edges[j].From
		}
		return s.Edges[i].To < s.Edges[j].To
	})
}

// ---------------------------------------------------------------------------
// Errors

var (
	// ErrLockEmpty is an empty lock file, which is not a valid empty graph.
	ErrLockEmpty = errors.New("lock file is empty")
	// ErrLockMissing distinguishes an absent file from an empty or valid lock.
	ErrLockMissing = errors.New("lock file is missing")
	// ErrLockMissingHeader is a file whose first record is not a lock header;
	// in particular it covers a headerless lock written by an older toolchain,
	// which has no decoder and must be regenerated by get.
	ErrLockMissingHeader = errors.New("lock file has no format header")
	// ErrLockUnsupportedFormat is a header naming a format no codec handles.
	ErrLockUnsupportedFormat = errors.New("unsupported lock format")
	// ErrLockUnknownResolver is a resolver whose meaning this build does not
	// know. It is never mapped onto a newer comparator.
	ErrLockUnknownResolver = errors.New("unknown lock resolver")
	// ErrLockKindMismatch is a header kind that does not match the caller.
	ErrLockKindMismatch = errors.New("lock kind mismatch")
	// ErrLockInvalid is a syntactically readable lock with invalid meaning.
	ErrLockInvalid = errors.New("invalid lock")
	// ErrLockPreservation is a migration that lost or changed meaning.
	ErrLockPreservation = errors.New("lock migration would change resolution meaning")
	// ErrLockUpgradeMissingStep is a gap in the registered migration path.
	ErrLockUpgradeMissingStep = errors.New("lock upgrade step is missing")
	// ErrLockUpgradeDuplicateStep is a registry conflict.
	ErrLockUpgradeDuplicateStep = errors.New("lock upgrade step is duplicated")
	// ErrLockUpgradeNeedsResolution marks a step that cannot be a format
	// migration without re-resolving dependencies.
	ErrLockUpgradeNeedsResolution = errors.New("lock upgrade needs a new resolution")
	// ErrLockUpgradeNeedsEvidence marks an EnrichPinned step waiting for
	// pinned commit evidence supplied by get.
	ErrLockUpgradeNeedsEvidence = errors.New("lock upgrade needs pinned evidence")
)

// ---------------------------------------------------------------------------
// IDs and digests

// LockSourceID is the Source record ID for an identity. The encoding is a
// fixed-order, length-prefixed tuple, independent of Go map order or JSON.
func LockSourceID(id SourceIdentity) string {
	return lockDigest("source-v1", id.Name, id.URL, id.Subdir)
}

// LockResolutionID is the Resolution record ID for a source and raw request.
func LockResolutionID(sourceID, requested string) string {
	return lockDigest("resolution-v1", sourceID, requested)
}

func lockDigest(tag string, parts ...string) string {
	h := sha256.New()
	writeDigestPart(h, tag)
	for _, p := range parts {
		writeDigestPart(h, p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeDigestPart(h interface{ Write([]byte) (int, error) }, s string) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(s)))
	_, _ = h.Write(n[:])
	_, _ = h.Write([]byte(s))
}

func writeDigestBytes(h interface{ Write([]byte) (int, error) }, b []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	_, _ = h.Write(n[:])
	_, _ = h.Write(b)
}

// LockInputsDigest hashes the resolution inputs a lock header records: the
// resolver identity, the root/member manifest bytes in the order given and,
// when selection read them, the local override declarations. Absolute
// checkout paths, mtimes and cache locations never enter the digest, so a
// lock stays portable across checkouts.
func LockInputsDigest(resolver string, manifests [][]byte, local []byte) string {
	h := sha256.New()
	writeDigestPart(h, "inputs-v1")
	writeDigestPart(h, "resolver")
	writeDigestPart(h, resolver)
	writeDigestPart(h, "manifests")
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(manifests)))
	_, _ = h.Write(count[:])
	for _, m := range manifests {
		writeDigestBytes(h, m)
	}
	if local != nil {
		writeDigestPart(h, "local")
		writeDigestBytes(h, local)
	}
	return "h1:" + hex.EncodeToString(h.Sum(nil))
}

// EmptyLockState is a zero-dependency document in the published format. A
// lock with no Lock records still keeps its header: losing the file is not
// the same statement as recording that nothing was selected.
func EmptyLockState(kind LockKind) LockState {
	return LockState{Kind: kind, Format: LockFormat1, Resolver: LockResolverOrderedV1}
}

// SelectedLockState builds the document the current fetch path can honestly
// write: the header plus one Lock record per selected entry. The fetch path
// stores each entry's raw request, full commit, content hash and source URL
// but no commit time and no base-tag evidence, so complete Source and
// Resolution records cannot be produced without inventing fields. Callers
// with that evidence build a LockState directly instead.
func SelectedLockState(inputs string, entries map[string]LockEntry) (LockState, error) {
	state := EmptyLockState(LockKindModule)
	state.Inputs = inputs
	for _, e := range entries {
		commit, err := ParseCommitID(e.Commit)
		if err != nil {
			return LockState{}, fmt.Errorf("%w: lock %q commit: %v", ErrLockInvalid, e.Require().Key(), err)
		}
		state.Locks = append(state.Locks, LockBinding{
			Name:      e.Name,
			Alias:     e.Alias,
			Requested: e.Version,
			Commit:    commit,
			Hash:      e.Hash,
			URL:       e.URL,
		})
	}
	state.SortRecords()
	if err := state.Validate(); err != nil {
		return LockState{}, err
	}
	return state, nil
}

// LockBytesDigest labels the original lock bytes in an upgrade plan.
func LockBytesDigest(src []byte) string {
	sum := sha256.Sum256(src)
	return "h1:" + hex.EncodeToString(sum[:])
}

// ValidLockH1 reports a well-formed `h1:` SHA-256 digest: lowercase hex and
// exactly the digest width. Different hash schemes are never accepted here.
func ValidLockH1(s string) bool {
	if !strings.HasPrefix(s, "h1:") || len(s) != 3+64 {
		return false
	}
	for _, c := range s[3:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func validLockID(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ParseLockTime reads the lock's UTC time representation exactly; offsets,
// fractional seconds and local zones are rejected rather than normalized.
func ParseLockTime(s string) (time.Time, error) {
	const layout = "2006-01-02T15:04:05Z"
	t, err := time.Parse(layout, s)
	if err != nil || t.Format(layout) != s {
		return time.Time{}, fmt.Errorf("lock time %q is not UTC `YYYY-MM-DDTHH:mm:ssZ`", s)
	}
	return t.UTC(), nil
}

// FormatLockTime renders a time in the single representation locks store.
func FormatLockTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// ---------------------------------------------------------------------------
// Semantic validation

// Validate checks that s has a coherent meaning. Record IDs are recomputed:
// a lock whose IDs do not derive from its records is corrupt, not merely
// differently formatted.
func (s LockState) Validate() error { return s.validate(true) }

// validateMeaning checks references and integrity without requiring IDs to be
// the current derivation. A format migration may reconstruct IDs; it must
// still preserve the meaning they point at.
func (s LockState) validateMeaning() error { return s.validate(false) }

func (s LockState) validate(checkIDs bool) error {
	if !s.Kind.Valid() {
		return fmt.Errorf("%w: unknown kind %d", ErrLockInvalid, uint8(s.Kind))
	}
	if s.Format <= 0 {
		return fmt.Errorf("%w: format must be a positive integer", ErrLockInvalid)
	}
	if s.Resolver != LockResolverOrderedV1 {
		return fmt.Errorf("%w: %q", ErrLockUnknownResolver, s.Resolver)
	}
	if !ValidLockH1(s.Inputs) {
		return fmt.Errorf("%w: inputs %q is not an h1: SHA-256 digest", ErrLockInvalid, s.Inputs)
	}
	sources, err := s.sourceMaps(checkIDs)
	if err != nil {
		return err
	}
	if err := s.validateResolutions(sources, checkIDs); err != nil {
		return err
	}
	if err := s.validateBindings(sources); err != nil {
		return err
	}
	return nil
}

func (s LockState) sourceMaps(checkIDs bool) (map[string]LockSource, error) {
	byID := make(map[string]LockSource, len(s.Sources))
	byIdentity := make(map[string]string, len(s.Sources))
	for _, src := range s.Sources {
		if !validLockID(src.ID) {
			return nil, fmt.Errorf("%w: source id %q is not 64 lowercase hex digits", ErrLockInvalid, src.ID)
		}
		if err := src.Identity.Validate(); err != nil {
			return nil, fmt.Errorf("%w: source %s: %v", ErrLockInvalid, src.ID, err)
		}
		if checkIDs {
			if want := LockSourceID(src.Identity); src.ID != want {
				return nil, fmt.Errorf("%w: source id %s does not match its identity (want %s)", ErrLockInvalid, src.ID, want)
			}
		}
		if _, dup := byID[src.ID]; dup {
			return nil, fmt.Errorf("%w: source id %s is duplicated", ErrLockInvalid, src.ID)
		}
		key := sourceIdentityKey(src.Identity)
		if _, dup := byIdentity[key]; dup {
			return nil, fmt.Errorf("%w: source identity %s is recorded twice", ErrLockInvalid, describeSource(src.Identity))
		}
		byID[src.ID] = src
		byIdentity[key] = src.ID
	}
	return byID, nil
}

func (s LockState) validateResolutions(sources map[string]LockSource, checkIDs bool) error {
	resIDs := make(map[string]bool, len(s.Resolutions))
	resKeys := make(map[string]bool, len(s.Resolutions))
	versionCommits := make(map[string]string, len(s.Resolutions))
	for _, res := range s.Resolutions {
		where := fmt.Sprintf("resolution %s", res.ID)
		if !validLockID(res.ID) {
			return fmt.Errorf("%w: %s: id is not 64 lowercase hex digits", ErrLockInvalid, where)
		}
		if resIDs[res.ID] {
			return fmt.Errorf("%w: %s is duplicated", ErrLockInvalid, where)
		}
		src, ok := sources[res.SourceID]
		if !ok {
			return fmt.Errorf("%w: %s references missing source %s", ErrLockInvalid, where, res.SourceID)
		}
		if res.Requested == "" {
			return fmt.Errorf("%w: %s has an empty requested revision", ErrLockInvalid, where)
		}
		if err := ValidVersion(res.Requested); err != nil {
			return fmt.Errorf("%w: %s: requested %q: %v", ErrLockInvalid, where, res.Requested, err)
		}
		if res.Version.IsZero() {
			return fmt.Errorf("%w: %s has no canonical version", ErrLockInvalid, where)
		}
		if err := CheckVersionForName(src.Identity.Name, res.Version.String()); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrLockInvalid, where, err)
		}
		if err := res.Commit.Validate(); err != nil {
			return fmt.Errorf("%w: %s: commit: %v", ErrLockInvalid, where, err)
		}
		if res.Time.IsZero() {
			return fmt.Errorf("%w: %s has no commit time", ErrLockInvalid, where)
		}
		if res.Time.Nanosecond() != 0 {
			return fmt.Errorf("%w: %s commit time has sub-second precision", ErrLockInvalid, where)
		}
		if _, offset := res.Time.Zone(); offset != 0 {
			return fmt.Errorf("%w: %s commit time is not UTC", ErrLockInvalid, where)
		}
		if !ValidLockH1(res.Manifest) {
			return fmt.Errorf("%w: %s: manifest %q is not an h1: SHA-256 digest", ErrLockInvalid, where, res.Manifest)
		}
		if checkIDs {
			if want := LockResolutionID(res.SourceID, res.Requested); res.ID != want {
				return fmt.Errorf("%w: %s does not match (source, requested) (want %s)", ErrLockInvalid, where, want)
			}
		}
		key := res.SourceID + "\x00" + res.Requested
		if resKeys[key] {
			return fmt.Errorf("%w: resolution for source %s request %q is duplicated", ErrLockInvalid, res.SourceID, res.Requested)
		}
		resKeys[key] = true
		resIDs[res.ID] = true

		cv := res.SourceID + "\x00" + res.Version.String()
		if prev, ok := versionCommits[cv]; ok && prev != res.Commit.String() {
			return fmt.Errorf("%w: source %s version %s resolves to both %s and %s", ErrLockInvalid, res.SourceID, res.Version, prev, res.Commit)
		}
		versionCommits[cv] = res.Commit.String()

		if err := validateRequestedResolution(src, res); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrLockInvalid, where, err)
		}
	}
	return nil
}

func validateRequestedResolution(src LockSource, res LockResolution) error {
	requested, err := ParseRequestVersion(res.Requested)
	if err != nil {
		return fmt.Errorf("requested %q: %v", res.Requested, err)
	}
	if res.Tag != "" && requested.Kind != RequestVersionTag {
		return fmt.Errorf("tag %q is recorded for a %s request", res.Tag, requested.Kind)
	}
	version := res.Version.String()
	if p, err := ParsePseudoVersion(version); err == nil {
		if err := ValidatePseudoVersion(src.Identity.Name, p, res.Commit, res.Base); err != nil {
			return err
		}
		if !p.Time.Equal(res.Time) {
			return fmt.Errorf("pseudo-version %s time %s does not match commit time %s", version, p.Time.Format(pseudoTimeLayout), FormatLockTime(res.Time))
		}
	} else if res.Base != nil {
		return fmt.Errorf("version %s is not a pseudo-version but records base evidence", version)
	}

	switch requested.Kind {
	case RequestVersionCommit:
		if !strings.HasPrefix(res.Commit.Hex, requested.CommitHex) {
			return fmt.Errorf("requested hash %s is not a prefix of commit %s", requested.CommitHex, res.Commit)
		}
	case RequestVersionPseudo:
		if version != requested.Pseudo.Version.String() {
			return fmt.Errorf("requested pseudo-version %s was rewritten to %s", requested.Raw, version)
		}
	case RequestVersionTag:
		if err := CheckVersionForName(src.Identity.Name, requested.Tag.Version.String()); err != nil {
			return err
		}
		return validateTagFields(src, res, requested.Raw, version)
	}
	return nil
}

func validateTagFields(src LockSource, res LockResolution, requested, version string) error {
	if res.Tag == "" {
		return fmt.Errorf("tag request %q must record its tag", requested)
	}
	if !tagMatchesSource(src, res.Tag) {
		return fmt.Errorf("tag %q does not belong to source %s", res.Tag, describeSource(src.Identity))
	}
	sansBuild, _, _ := strings.Cut(requested, "+")
	if sansBuild == version {
		return nil
	}
	if res.Base != nil && requested != version {
		if res.Base.Tag == res.Tag && res.Base.Version.String() == sansBuild {
			return nil
		}
	}
	return fmt.Errorf("tag request %q resolved to %s without matching base evidence", requested, version)
}

func tagMatchesSource(src LockSource, tag string) bool {
	version := tag
	if src.Identity.Subdir != "" {
		prefix := src.Identity.Subdir + "/"
		if !strings.HasPrefix(tag, prefix) {
			return false
		}
		version = strings.TrimPrefix(tag, prefix)
	}
	if version == "" {
		return false
	}
	for _, c := range version {
		if !(c == 'v' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '.' || c == '-' || c == '+') {
			return false
		}
	}
	return version[0] == 'v'
}

func (s LockState) validateBindings(sources map[string]LockSource) error {
	resByID := make(map[string]LockResolution, len(s.Resolutions))
	for _, res := range s.Resolutions {
		resByID[res.ID] = res
	}
	keys := make(map[string]bool, len(s.Locks))
	versionCommits := make(map[string]string, len(s.Locks))
	for _, lk := range s.Locks {
		key := lk.Key()
		if key == "" {
			return fmt.Errorf("%w: a lock record has neither a name nor an alias", ErrLockInvalid)
		}
		if keys[key] {
			return fmt.Errorf("%w: effective key %q is locked twice", ErrLockInvalid, key)
		}
		keys[key] = true
		if err := ValidModuleName(lk.Name); err != nil {
			return fmt.Errorf("%w: lock %q: %v", ErrLockInvalid, key, err)
		}
		if lk.Alias != "" {
			if err := ValidAlias(lk.Alias); err != nil {
				return fmt.Errorf("%w: lock %q: %v", ErrLockInvalid, key, err)
			}
		}
		if err := lk.Commit.Validate(); err != nil {
			return fmt.Errorf("%w: lock %q commit: %v", ErrLockInvalid, key, err)
		}
		if !ValidLockH1(lk.Hash) {
			return fmt.Errorf("%w: lock %q hash %q is not an h1: SHA-256 digest", ErrLockInvalid, key, lk.Hash)
		}
		if !lk.complete() {
			if lk.Version.IsZero() {
				if lk.Requested == "" {
					return fmt.Errorf("%w: lock %q has no version", ErrLockInvalid, key)
				}
			} else {
				return fmt.Errorf("%w: lock %q records both a canonical version and a raw request", ErrLockInvalid, key)
			}
			if err := validateSelectedRequest(lk); err != nil {
				return fmt.Errorf("%w: lock %q: %v", ErrLockInvalid, key, err)
			}
			continue
		}
		if lk.Requested != "" {
			return fmt.Errorf("%w: lock %q records a raw request beside resolution evidence", ErrLockInvalid, key)
		}
		if lk.SourceID == "" || lk.ResolutionID == "" {
			return fmt.Errorf("%w: lock %q must reference both a source and a resolution", ErrLockInvalid, key)
		}
		src, ok := sources[lk.SourceID]
		if !ok {
			return fmt.Errorf("%w: lock %q references missing source %s", ErrLockInvalid, key, lk.SourceID)
		}
		res, ok := resByID[lk.ResolutionID]
		if !ok {
			return fmt.Errorf("%w: lock %q references missing resolution %s", ErrLockInvalid, key, lk.ResolutionID)
		}
		if lk.Name != src.Identity.Name {
			return fmt.Errorf("%w: lock %q names %s but its source names %s", ErrLockInvalid, key, lk.Name, src.Identity.Name)
		}
		if lk.SourceID != res.SourceID {
			return fmt.Errorf("%w: lock %q and resolution %s disagree about the source", ErrLockInvalid, key, res.ID)
		}
		if lk.Version.IsZero() || lk.Version.String() != res.Version.String() {
			return fmt.Errorf("%w: lock %q version %s does not match resolution %s version %s", ErrLockInvalid, key, lk.Version, res.ID, res.Version)
		}
		if err := CheckVersionForName(lk.Name, lk.Version.String()); err != nil {
			return fmt.Errorf("%w: lock %q: %v", ErrLockInvalid, key, err)
		}
		if lk.Commit.String() != res.Commit.String() {
			return fmt.Errorf("%w: lock %q commit %s does not match resolution %s commit %s", ErrLockInvalid, key, lk.Commit, res.ID, res.Commit)
		}
		if lk.URL != "" && lk.URL != src.Identity.URL {
			return fmt.Errorf("%w: lock %q records url %q but its source records %q", ErrLockInvalid, key, lk.URL, src.Identity.URL)
		}
		cv := lk.SourceID + "\x00" + lk.Version.String()
		if prev, ok := versionCommits[cv]; ok && prev != lk.Commit.String() {
			return fmt.Errorf("%w: source %s version %s is locked to both %s and %s", ErrLockInvalid, lk.SourceID, lk.Version, prev, lk.Commit)
		}
		versionCommits[cv] = lk.Commit.String()
	}
	return nil
}

// validateSelectedRequest checks the evidence a selected-only Lock record can
// actually prove: request syntax, path-major compatibility, a commit prefix
// for a hash request and the revision suffix of a pseudo-version. A tag's
// commit and a based pseudo-version's base are the evidence that a Resolution
// record would carry, and a selected-only record must not pretend to have it.
func validateSelectedRequest(lk LockBinding) error {
	requested, err := ParseRequestVersion(lk.Requested)
	if err != nil {
		return err
	}
	switch requested.Kind {
	case RequestVersionCommit:
		if !strings.HasPrefix(lk.Commit.Hex, requested.CommitHex) {
			return fmt.Errorf("requested hash %s is not a prefix of commit %s", requested.CommitHex, lk.Commit)
		}
	case RequestVersionPseudo:
		if requested.Pseudo.Rev != lk.Commit.Short() {
			return fmt.Errorf("pseudo-version %s revision %q does not match commit %s", requested.Raw, requested.Pseudo.Rev, lk.Commit)
		}
		if err := CheckVersionForName(lk.Name, requested.Pseudo.Version.String()); err != nil {
			return err
		}
	case RequestVersionTag:
		if err := CheckVersionForName(lk.Name, requested.Raw); err != nil {
			return err
		}
	}
	return nil
}

func sourceIdentityKey(id SourceIdentity) string {
	return id.Name + "\x00" + id.URL + "\x00" + id.Subdir
}

func describeSource(id SourceIdentity) string {
	return fmt.Sprintf("%s (%s, subdir %q)", id.Name, id.URL, id.Subdir)
}
