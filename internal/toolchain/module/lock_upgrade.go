package module

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// LockUpgradeClass says how much a format step is allowed to do. Pure steps
// use only the bytes already in the lock; EnrichPinned steps may add
// evidence for an already pinned commit, but only when get supplies it;
// NeedsResolution steps are not migrations at all.
type LockUpgradeClass uint8

const (
	LockUpgradePure LockUpgradeClass = iota + 1
	LockUpgradeEnrichPinned
	LockUpgradeNeedsResolution
)

func (c LockUpgradeClass) String() string {
	switch c {
	case LockUpgradePure:
		return "Pure"
	case LockUpgradeEnrichPinned:
		return "EnrichPinned"
	case LockUpgradeNeedsResolution:
		return "NeedsResolution"
	default:
		return fmt.Sprintf("LockUpgradeClass(%d)", uint8(c))
	}
}

func (c LockUpgradeClass) valid() bool {
	return c == LockUpgradePure || c == LockUpgradeEnrichPinned || c == LockUpgradeNeedsResolution
}

// PinnedEvidenceKind names one piece of information EnrichPinned may fill in
// for a recorded source and full commit.
type PinnedEvidenceKind string

const (
	PinnedEvidenceManifest PinnedEvidenceKind = "manifest"
	PinnedEvidenceHash     PinnedEvidenceKind = "hash"
	PinnedEvidenceTime     PinnedEvidenceKind = "time"
)

func (k PinnedEvidenceKind) valid() bool {
	return k == PinnedEvidenceManifest || k == PinnedEvidenceHash || k == PinnedEvidenceTime
}

// LockPinnedEvidence is the integrity already present for a pinned commit.
// Enrichment may add missing fields but must not replace these values with a
// recomputed hash or a later observation.
type LockPinnedEvidence struct {
	Manifest string
	Hash     string
	Time     time.Time
}

// PinnedEvidenceRequest is a typed request for get to fill in information
// about an already locked source and full commit. It never asks for a tag,
// branch or "latest" query.
type PinnedEvidenceRequest struct {
	SourceID string
	Source   SourceIdentity
	Commit   CommitID
	Needs    []PinnedEvidenceKind
	Existing LockPinnedEvidence
}

// PinnedEvidence is the validated answer to a PinnedEvidenceRequest.
type PinnedEvidence struct {
	SourceID string
	Commit   CommitID
	Manifest string
	Hash     string
	Time     time.Time
}

// LockUpgradeStep is one edge in a migration path: exactly one next format.
// Transform must not mutate its input; the registry calls it with a clone and
// verifies the original is unchanged.
type LockUpgradeStep struct {
	Kind      LockKind
	From, To  int
	ID        string
	Class     LockUpgradeClass
	Reason    string
	Transform func(in LockState, evidence []PinnedEvidence) (LockState, error)
	Evidence  func(in LockState) ([]PinnedEvidenceRequest, error)
}

// LockUpgradePlan is a sequence of steps from From to To, plus any pinned
// evidence the next step needs and a digest of the original lock bytes.
type LockUpgradePlan struct {
	Kind          LockKind
	From, To      int
	Steps         []string
	Evidence      []PinnedEvidenceRequest
	SourceDigest  string
	NeedsEvidence bool
}

// LockUpgradeReport distinguishes a representation-only upgrade from a
// dependency change. WriteFormat defaults to the in-memory target; a writer
// replaces it with ChooseLockWriteFormat using the project's declared minimum
// toolchain. It is never chosen by a reader.
type LockUpgradeReport struct {
	From              int
	To                int
	Steps             []string
	WriteFormat       int
	ResolutionChanged bool
	MetadataAdded     bool
}

type lockUpgradeKey struct {
	Kind LockKind
	From int
}

// LockUpgradeRegistry maps (kind, from format) to one next step. Steps are
// registered explicitly; an unknown format is never read by another codec.
type LockUpgradeRegistry struct {
	steps map[lockUpgradeKey]LockUpgradeStep
	ids   map[LockKind]map[string]bool
}

// NewLockUpgradeRegistry returns an empty registry. The product registry has
// no step until a later format exists; tests construct synthetic registries.
func NewLockUpgradeRegistry() *LockUpgradeRegistry {
	return &LockUpgradeRegistry{
		steps: map[lockUpgradeKey]LockUpgradeStep{},
		ids:   map[LockKind]map[string]bool{},
	}
}

// Register adds one step. The next format must be exactly From+1, so skips,
// cycles and downgrades are rejected at registration, as is a duplicate
// (kind, from) or duplicate step ID.
func (r *LockUpgradeRegistry) Register(step LockUpgradeStep) error {
	if !step.Kind.Valid() {
		return fmt.Errorf("lock upgrade step %q: unknown kind %d", step.ID, uint8(step.Kind))
	}
	if step.From < LockFormat1 {
		return fmt.Errorf("lock upgrade step %q: from format %d is before the published format %d", step.ID, step.From, LockFormat1)
	}
	if step.To != step.From+1 {
		return fmt.Errorf("lock upgrade step %q: %d -> %d is not exactly one format forward", step.ID, step.From, step.To)
	}
	if step.ID == "" {
		return fmt.Errorf("lock upgrade step for %s %d has no ID", step.Kind, step.From)
	}
	if !step.Class.valid() {
		return fmt.Errorf("lock upgrade step %q has unknown classification %d", step.ID, uint8(step.Class))
	}
	switch step.Class {
	case LockUpgradePure:
		if step.Transform == nil || step.Evidence != nil || step.Reason != "" {
			return fmt.Errorf("lock upgrade step %q: Pure needs Transform and no Evidence/Reason", step.ID)
		}
	case LockUpgradeEnrichPinned:
		if step.Transform == nil || step.Evidence == nil || step.Reason != "" {
			return fmt.Errorf("lock upgrade step %q: EnrichPinned needs Transform and Evidence, and no Reason", step.ID)
		}
	case LockUpgradeNeedsResolution:
		if step.Transform != nil || step.Evidence != nil || step.Reason == "" {
			return fmt.Errorf("lock upgrade step %q: NeedsResolution needs a Reason and no Transform/Evidence", step.ID)
		}
	}
	key := lockUpgradeKey{Kind: step.Kind, From: step.From}
	if _, dup := r.steps[key]; dup {
		return fmt.Errorf("%w: %s %d -> %d", ErrLockUpgradeDuplicateStep, step.Kind, step.From, step.To)
	}
	for id := range r.ids[step.Kind] {
		if id == step.ID {
			return fmt.Errorf("%w: step ID %q", ErrLockUpgradeDuplicateStep, step.ID)
		}
	}
	r.steps[key] = step
	if r.ids[step.Kind] == nil {
		r.ids[step.Kind] = map[string]bool{}
	}
	r.ids[step.Kind][step.ID] = true
	return nil
}

// Step returns the registered next step for kind and from.
func (r *LockUpgradeRegistry) Step(kind LockKind, from int) (LockUpgradeStep, bool) {
	step, ok := r.steps[lockUpgradeKey{Kind: kind, From: from}]
	return step, ok
}

// Plan walks the registered path from state.Format to target, applying Pure
// steps in memory so later Evidence functions see the right state. It stops
// at the first EnrichPinned step and returns its typed evidence requests.
func (r *LockUpgradeRegistry) Plan(state LockState, target int, original []byte) (LockUpgradePlan, error) {
	if !state.Kind.Valid() {
		return LockUpgradePlan{}, fmt.Errorf("%w: %d", ErrLockKindMismatch, uint8(state.Kind))
	}
	if target < state.Format {
		return LockUpgradePlan{}, fmt.Errorf("%w: cannot downgrade %s from %d to %d", ErrLockUpgradeMissingStep, state.Kind, state.Format, target)
	}
	plan := LockUpgradePlan{Kind: state.Kind, From: state.Format, To: target}
	if original != nil {
		plan.SourceDigest = LockBytesDigest(original)
	}
	current := state.Clone()
	for current.Format < target {
		step, ok := r.Step(state.Kind, current.Format)
		if !ok {
			return LockUpgradePlan{}, fmt.Errorf("%w: %s %d -> %d", ErrLockUpgradeMissingStep, state.Kind, current.Format, current.Format+1)
		}
		plan.Steps = append(plan.Steps, step.ID)
		switch step.Class {
		case LockUpgradePure:
			next, err := applyPureStep(step, current)
			if err != nil {
				return LockUpgradePlan{}, err
			}
			current = next
		case LockUpgradeEnrichPinned:
			requests, err := step.Evidence(current)
			if err != nil {
				return LockUpgradePlan{}, fmt.Errorf("lock upgrade step %q: %w", step.ID, err)
			}
			if len(requests) == 0 {
				return LockUpgradePlan{}, fmt.Errorf("lock upgrade step %q needs evidence but listed none", step.ID)
			}
			if err := validateEvidenceRequests(requests); err != nil {
				return LockUpgradePlan{}, fmt.Errorf("lock upgrade step %q: %w", step.ID, err)
			}
			plan.Evidence = append(plan.Evidence, requests...)
			plan.NeedsEvidence = true
			plan.To = step.To
			return plan, nil
		case LockUpgradeNeedsResolution:
			return LockUpgradePlan{}, fmt.Errorf("%w: step %s: %s", ErrLockUpgradeNeedsResolution, step.ID, step.Reason)
		}
	}
	return plan, nil
}

// Upgrade applies a purely representational path. An EnrichPinned step needs
// ApplyPlan with evidence; this helper refuses to guess.
func (r *LockUpgradeRegistry) Upgrade(state LockState, target int, original []byte) (LockState, LockUpgradeReport, error) {
	plan, err := r.Plan(state, target, original)
	if err != nil {
		return state, LockUpgradeReport{}, err
	}
	if plan.NeedsEvidence {
		return state, LockUpgradeReport{}, fmt.Errorf("%w: %s needs %d pinned evidence request(s)", ErrLockUpgradeNeedsEvidence, plan.Kind, len(plan.Evidence))
	}
	out, err := r.ApplyPlan(state, plan, nil)
	if err != nil {
		return state, LockUpgradeReport{}, err
	}
	return out, LockUpgradeReport{
		From:        plan.From,
		To:          out.Format,
		Steps:       append([]string(nil), plan.Steps...),
		WriteFormat: out.Format,
	}, nil
}

// ApplyPlan executes plan.Steps. Every step's output must preserve the
// semantics of its input, and the input state must remain untouched.
func (r *LockUpgradeRegistry) ApplyPlan(state LockState, plan LockUpgradePlan, evidence []PinnedEvidence) (LockState, error) {
	if state.Kind != plan.Kind || state.Format != plan.From {
		return state, fmt.Errorf("lock upgrade plan starts at %s %d, state is %s %d", plan.Kind, plan.From, state.Kind, state.Format)
	}
	current := state.Clone()
	for _, id := range plan.Steps {
		step, ok := r.Step(current.Kind, current.Format)
		if !ok {
			return state, fmt.Errorf("%w: %s %d -> %d", ErrLockUpgradeMissingStep, current.Kind, current.Format, current.Format+1)
		}
		if step.ID != id {
			return state, fmt.Errorf("lock upgrade plan expects step %q at %s %d but registry has %q", id, current.Kind, current.Format, step.ID)
		}
		var matched []PinnedEvidence
		switch step.Class {
		case LockUpgradePure:
			if len(evidence) != 0 {
				return state, fmt.Errorf("lock upgrade step %q is Pure but evidence was supplied", step.ID)
			}
		case LockUpgradeEnrichPinned:
			requests, err := step.Evidence(current)
			if err != nil {
				return state, fmt.Errorf("lock upgrade step %q: %w", step.ID, err)
			}
			matched, err = matchPinnedEvidence(requests, evidence)
			if err != nil {
				return state, fmt.Errorf("lock upgrade step %q: %w", step.ID, err)
			}
		case LockUpgradeNeedsResolution:
			return state, fmt.Errorf("%w: step %s: %s", ErrLockUpgradeNeedsResolution, step.ID, step.Reason)
		}
		input := current.Clone()
		next, err := step.Transform(input, matched)
		if err != nil {
			return state, fmt.Errorf("lock upgrade step %q: %w", step.ID, err)
		}
		if !reflect.DeepEqual(input, current) {
			return state, fmt.Errorf("lock upgrade step %q mutated its input", step.ID)
		}
		if next.Kind != current.Kind || next.Format != step.To {
			return state, fmt.Errorf("lock upgrade step %q produced %s %d, want %s %d", step.ID, next.Kind, next.Format, current.Kind, step.To)
		}
		if next.Resolver != current.Resolver || next.Inputs != current.Inputs {
			return state, fmt.Errorf("lock upgrade step %q changed resolver or inputs", step.ID)
		}
		if err := ValidatePreservedBindings(current, next); err != nil {
			return state, fmt.Errorf("lock upgrade step %q: %w", step.ID, err)
		}
		current = next
	}
	if current.Format != plan.To {
		return state, fmt.Errorf("lock upgrade plan ended at %s %d, want %d", current.Kind, current.Format, plan.To)
	}
	return current, nil
}

func applyPureStep(step LockUpgradeStep, in LockState) (LockState, error) {
	input := in.Clone()
	out, err := step.Transform(input, nil)
	if err != nil {
		return in, fmt.Errorf("lock upgrade step %q: %w", step.ID, err)
	}
	if !reflect.DeepEqual(input, in) {
		return in, fmt.Errorf("lock upgrade step %q mutated its input", step.ID)
	}
	if out.Kind != in.Kind || out.Format != step.To {
		return in, fmt.Errorf("lock upgrade step %q produced %s %d, want %s %d", step.ID, out.Kind, out.Format, in.Kind, step.To)
	}
	if out.Resolver != in.Resolver || out.Inputs != in.Inputs {
		return in, fmt.Errorf("lock upgrade step %q changed resolver or inputs", step.ID)
	}
	if err := ValidatePreservedBindings(in, out); err != nil {
		return in, fmt.Errorf("lock upgrade step %q: %w", step.ID, err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Preservation

type lockSemantics struct {
	resolver    string
	inputs      string
	sources     map[string]string
	resolutions map[string]string
	locks       map[string]string
	edges       map[string]bool
}

// ValidatePreservedBindings rejects a migration that loses or changes a
// source, a request (including loser requests), a selected binding, an
// integrity value or a selected edge. Format, record IDs and redundant
// fields are representation and may change.
func ValidatePreservedBindings(before, after LockState) error {
	if before.Kind != after.Kind {
		return fmt.Errorf("%w: kind changed from %s to %s", ErrLockPreservation, before.Kind, after.Kind)
	}
	b, err := takeLockSemantics(before)
	if err != nil {
		return fmt.Errorf("%w: before: %v", ErrLockPreservation, err)
	}
	a, err := takeLockSemantics(after)
	if err != nil {
		return fmt.Errorf("%w: after: %v", ErrLockPreservation, err)
	}
	var diffs []string
	if b.resolver != a.resolver {
		diffs = append(diffs, fmt.Sprintf("resolver %q -> %q", b.resolver, a.resolver))
	}
	if b.inputs != a.inputs {
		diffs = append(diffs, fmt.Sprintf("inputs %q -> %q", b.inputs, a.inputs))
	}
	diffs = append(diffs, compareStringMaps("source", b.sources, a.sources)...)
	diffs = append(diffs, compareStringMaps("resolution", b.resolutions, a.resolutions)...)
	diffs = append(diffs, compareStringMaps("selected binding", b.locks, a.locks)...)
	for e := range b.edges {
		if !a.edges[e] {
			diffs = append(diffs, fmt.Sprintf("selected edge %s was dropped", e))
		}
	}
	for e := range a.edges {
		if !b.edges[e] {
			diffs = append(diffs, fmt.Sprintf("selected edge %s was added", e))
		}
	}
	if len(diffs) > 0 {
		sort.Strings(diffs)
		return fmt.Errorf("%w: %s", ErrLockPreservation, strings.Join(diffs, "; "))
	}
	return nil
}

func takeLockSemantics(state LockState) (lockSemantics, error) {
	if err := state.validateMeaning(); err != nil {
		return lockSemantics{}, err
	}
	out := lockSemantics{
		resolver:    state.Resolver,
		inputs:      state.Inputs,
		sources:     map[string]string{},
		resolutions: map[string]string{},
		locks:       map[string]string{},
		edges:       map[string]bool{},
	}
	sourceKey := map[string]string{}
	for _, src := range state.Sources {
		key := sourceIdentityKey(src.Identity)
		sourceKey[src.ID] = key
		out.sources[key] = describeSource(src.Identity)
	}
	for _, res := range state.Resolutions {
		key := sourceKey[res.SourceID] + "\x00" + res.Requested
		var base string
		if res.Base != nil {
			base = res.Base.Version.String() + "\x00" + res.Base.Tag + "\x00" + res.Base.Commit.String()
		}
		out.resolutions[key] = strings.Join([]string{
			res.Version.String(),
			res.Commit.String(),
			FormatLockTime(res.Time),
			res.Manifest,
			res.Tag,
			base,
		}, "\x00")
	}
	for _, lk := range state.Locks {
		key := lk.Key()
		out.locks[key] = strings.Join([]string{
			lk.Name,
			lk.Alias,
			sourceKey[lk.SourceID],
			lk.SelectedVersion(),
			lk.Commit.String(),
			lk.Hash,
		}, "\x00")
	}
	for _, edge := range state.Edges {
		out.edges[edge.From+"\x00"+edge.To] = true
	}
	return out, nil
}

func compareStringMaps(what string, before, after map[string]string) []string {
	var diffs []string
	for key, val := range before {
		got, ok := after[key]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s %q was dropped", what, key))
			continue
		}
		if got != val {
			diffs = append(diffs, fmt.Sprintf("%s %q changed from %q to %q", what, key, val, got))
		}
	}
	for key := range after {
		if _, ok := before[key]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s %q was added", what, key))
		}
	}
	return diffs
}

// ---------------------------------------------------------------------------
// Pinned evidence

func validateEvidenceRequests(requests []PinnedEvidenceRequest) error {
	seen := map[string]bool{}
	for _, req := range requests {
		if !validLockID(req.SourceID) {
			return fmt.Errorf("%w: evidence request source id %q is not 64 lowercase hex digits", ErrLockInvalid, req.SourceID)
		}
		if err := req.Source.Validate(); err != nil {
			return fmt.Errorf("%w: evidence request source: %v", ErrLockInvalid, err)
		}
		if err := req.Commit.Validate(); err != nil {
			return fmt.Errorf("%w: evidence request commit: %v", ErrLockInvalid, err)
		}
		if len(req.Needs) == 0 {
			return fmt.Errorf("%w: evidence request for %s has no needed fields", ErrLockInvalid, req.Commit)
		}
		needs := map[PinnedEvidenceKind]bool{}
		for _, need := range req.Needs {
			if !need.valid() {
				return fmt.Errorf("%w: evidence request for %s has unknown need %q", ErrLockInvalid, req.Commit, need)
			}
			if needs[need] {
				return fmt.Errorf("%w: evidence request for %s repeats %q", ErrLockInvalid, req.Commit, need)
			}
			needs[need] = true
		}
		key := req.SourceID + "\x00" + req.Commit.String()
		if seen[key] {
			return fmt.Errorf("%w: evidence request for %s is duplicated", ErrLockInvalid, req.Commit)
		}
		seen[key] = true
		if req.Existing.Manifest != "" && !ValidLockH1(req.Existing.Manifest) {
			return fmt.Errorf("%w: existing manifest digest %q is invalid", ErrLockInvalid, req.Existing.Manifest)
		}
		if req.Existing.Hash != "" && !ValidLockH1(req.Existing.Hash) {
			return fmt.Errorf("%w: existing hash %q is invalid", ErrLockInvalid, req.Existing.Hash)
		}
	}
	return nil
}

func matchPinnedEvidence(requests []PinnedEvidenceRequest, evidence []PinnedEvidence) ([]PinnedEvidence, error) {
	if err := validateEvidenceRequests(requests); err != nil {
		return nil, err
	}
	byKey := map[string]PinnedEvidence{}
	for _, ev := range evidence {
		if !validLockID(ev.SourceID) {
			return nil, fmt.Errorf("%w: evidence source id %q is invalid", ErrLockInvalid, ev.SourceID)
		}
		if err := ev.Commit.Validate(); err != nil {
			return nil, fmt.Errorf("%w: evidence commit: %v", ErrLockInvalid, err)
		}
		key := ev.SourceID + "\x00" + ev.Commit.String()
		if _, dup := byKey[key]; dup {
			return nil, fmt.Errorf("%w: evidence for %s is duplicated", ErrLockInvalid, ev.Commit)
		}
		byKey[key] = ev
	}
	out := make([]PinnedEvidence, 0, len(requests))
	for _, req := range requests {
		key := req.SourceID + "\x00" + req.Commit.String()
		ev, ok := byKey[key]
		if !ok {
			return nil, fmt.Errorf("%w: %s at %s", ErrLockUpgradeNeedsEvidence, req.Source.Name, req.Commit)
		}
		delete(byKey, key)
		for _, need := range req.Needs {
			switch need {
			case PinnedEvidenceManifest:
				if !ValidLockH1(ev.Manifest) {
					return nil, fmt.Errorf("%w: manifest for %s is not an h1: digest", ErrLockInvalid, req.Commit)
				}
				if req.Existing.Manifest != "" && req.Existing.Manifest != ev.Manifest {
					return nil, fmt.Errorf("%w: manifest for %s changed from %s to %s", ErrLockPreservation, req.Commit, req.Existing.Manifest, ev.Manifest)
				}
			case PinnedEvidenceHash:
				if !ValidLockH1(ev.Hash) {
					return nil, fmt.Errorf("%w: hash for %s is not an h1: digest", ErrLockInvalid, req.Commit)
				}
				if req.Existing.Hash != "" && req.Existing.Hash != ev.Hash {
					return nil, fmt.Errorf("%w: hash for %s changed from %s to %s", ErrLockPreservation, req.Commit, req.Existing.Hash, ev.Hash)
				}
			case PinnedEvidenceTime:
				if ev.Time.IsZero() {
					return nil, fmt.Errorf("%w: time for %s is missing", ErrLockInvalid, req.Commit)
				}
				if !req.Existing.Time.IsZero() && !req.Existing.Time.Equal(ev.Time) {
					return nil, fmt.Errorf("%w: time for %s changed", ErrLockPreservation, req.Commit)
				}
			}
		}
		out = append(out, ev)
	}
	if len(byKey) != 0 {
		return nil, fmt.Errorf("%w: evidence supplied for an unrequested or unpinned revision", ErrLockInvalid)
	}
	return out, nil
}
