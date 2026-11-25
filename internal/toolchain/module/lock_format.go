package module

import (
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// LockFormat1 is the first published format of the ordered resolver lock.
// It is strict on both ends: an unknown record, field, duplicate header, bad
// reference or mismatched redundant value is an error, never a default.
const (
	LockFormat1           = 1
	LockResolverOrderedV1 = "ordered-v1"
)

var lockHeaderFields = map[string][]string{
	"ModuleLock":    {"format", "resolver", "inputs"},
	"WorkspaceLock": {"format", "resolver", "inputs"},
}

var lockRecordFields = map[string][]string{
	"ModuleLock":    {"format", "resolver", "inputs"},
	"WorkspaceLock": {"format", "resolver", "inputs"},
	"Source":        {"id", "name", "url", "subdir"},
	"Resolution":    {"id", "source", "requested", "version", "commit", "time", "manifest", "tag", "baseVersion", "baseTag", "baseCommit"},
	"Lock":          {"name", "alias", "source", "resolution", "version", "commit", "hash", "url"},
}

func lockFormatCodec() LockFormatCodec {
	return LockFormatCodec{
		Format: LockFormat1,
		Decode: decodeLockFormat,
		Encode: encodeLockFormat,
	}
}

func decodeLockFormat(name string, kind LockKind, header LockHeader, src []byte) (LockState, error) {
	if header.Format != LockFormat1 {
		return LockState{}, fmt.Errorf("%s: %w: %d", name, ErrLockUnsupportedFormat, header.Format)
	}
	f := token.NewFile(name, src)
	recs, ds := ParseRecords(f, lockRecordFields)
	if len(ds) > 0 {
		return LockState{}, fmt.Errorf("%s: %w: %s", name, ErrLockInvalid, diag.RenderAll(f, ds[:1]))
	}
	if len(recs) == 0 || recs[0].Kind != header.Kind.String() {
		return LockState{}, fmt.Errorf("%s: %w", name, ErrLockMissingHeader)
	}
	state := LockState{
		Kind:     header.Kind,
		Format:   header.Format,
		Resolver: header.Resolver,
		Inputs:   header.Inputs,
	}
	seenHeader := false
	for _, rec := range recs {
		if rec.Kind == "ModuleLock" || rec.Kind == "WorkspaceLock" {
			if seenHeader {
				return LockState{}, fmt.Errorf("%s: %w: header appears more than once", name, ErrLockInvalid)
			}
			seenHeader = true
			if rec.Kind != header.Kind.String() {
				return LockState{}, fmt.Errorf("%s: %w: %s header after %s", name, ErrLockKindMismatch, rec.Kind, header.Kind)
			}
			if rec.Fields["format"] != strconv.Itoa(header.Format) ||
				rec.Fields["resolver"] != header.Resolver ||
				rec.Fields["inputs"] != header.Inputs {
				return LockState{}, fmt.Errorf("%s: %w: header fields changed while reading", name, ErrLockInvalid)
			}
			continue
		}
		switch rec.Kind {
		case "Source":
			src, err := decodeLockSource(rec)
			if err != nil {
				return LockState{}, fmt.Errorf("%s: %w", name, err)
			}
			state.Sources = append(state.Sources, src)
		case "Resolution":
			res, err := decodeLockResolution(rec)
			if err != nil {
				return LockState{}, fmt.Errorf("%s: %w", name, err)
			}
			state.Resolutions = append(state.Resolutions, res)
		case "Lock":
			lk, err := decodeLockBinding(rec)
			if err != nil {
				return LockState{}, fmt.Errorf("%s: %w", name, err)
			}
			state.Locks = append(state.Locks, lk)
		default:
			return LockState{}, fmt.Errorf("%s: %w: unknown record %s", name, ErrLockInvalid, rec.Kind)
		}
	}
	if !seenHeader {
		return LockState{}, fmt.Errorf("%s: %w", name, ErrLockMissingHeader)
	}
	state.SortRecords()
	if err := state.Validate(); err != nil {
		return LockState{}, fmt.Errorf("%s: %w", name, err)
	}
	return state, nil
}

func decodeLockSource(rec Record) (LockSource, error) {
	fields := rec.Fields
	for _, required := range []string{"id", "name", "url"} {
		if _, ok := fields[required]; !ok {
			return LockSource{}, fmt.Errorf("%w: Source is missing field %s", ErrLockInvalid, required)
		}
	}
	id, err := NewSourceIdentity(fields["name"], fields["url"], fields["subdir"])
	if err != nil {
		return LockSource{}, fmt.Errorf("%w: Source: %v", ErrLockInvalid, err)
	}
	return LockSource{ID: fields["id"], Identity: id}, nil
}

func decodeLockResolution(rec Record) (LockResolution, error) {
	fields := rec.Fields
	for _, required := range []string{"id", "source", "requested", "version", "commit", "time", "manifest"} {
		if _, ok := fields[required]; !ok {
			return LockResolution{}, fmt.Errorf("%w: Resolution is missing field %s", ErrLockInvalid, required)
		}
	}
	version, err := ParseCanonicalVersion(fields["version"])
	if err != nil {
		return LockResolution{}, fmt.Errorf("%w: Resolution version: %v", ErrLockInvalid, err)
	}
	commit, err := ParseCommitID(fields["commit"])
	if err != nil {
		return LockResolution{}, fmt.Errorf("%w: Resolution commit: %v", ErrLockInvalid, err)
	}
	t, err := ParseLockTime(fields["time"])
	if err != nil {
		return LockResolution{}, fmt.Errorf("%w: Resolution time: %v", ErrLockInvalid, err)
	}
	res := LockResolution{
		ID:        fields["id"],
		SourceID:  fields["source"],
		Requested: fields["requested"],
		Version:   version,
		Commit:    commit,
		Time:      t,
		Manifest:  fields["manifest"],
		Tag:       fields["tag"],
	}
	baseFields := 0
	for _, name := range []string{"baseVersion", "baseTag", "baseCommit"} {
		if _, ok := fields[name]; ok {
			baseFields++
		}
	}
	if baseFields != 0 {
		if baseFields != 3 {
			return LockResolution{}, fmt.Errorf("%w: Resolution baseVersion, baseTag and baseCommit must appear together", ErrLockInvalid)
		}
		baseVersion, err := ParseCanonicalVersion(fields["baseVersion"])
		if err != nil {
			return LockResolution{}, fmt.Errorf("%w: Resolution baseVersion: %v", ErrLockInvalid, err)
		}
		baseCommit, err := ParseCommitID(fields["baseCommit"])
		if err != nil {
			return LockResolution{}, fmt.Errorf("%w: Resolution baseCommit: %v", ErrLockInvalid, err)
		}
		if fields["baseTag"] == "" {
			return LockResolution{}, fmt.Errorf("%w: Resolution baseTag is empty", ErrLockInvalid)
		}
		res.Base = &LockBase{Version: baseVersion, Tag: fields["baseTag"], Commit: baseCommit}
	}
	return res, nil
}

func decodeLockBinding(rec Record) (LockBinding, error) {
	fields := rec.Fields
	for _, required := range []string{"name", "version", "commit", "hash"} {
		if _, ok := fields[required]; !ok {
			return LockBinding{}, fmt.Errorf("%w: Lock is missing field %s", ErrLockInvalid, required)
		}
	}
	commit, err := ParseCommitID(fields["commit"])
	if err != nil {
		return LockBinding{}, fmt.Errorf("%w: Lock commit: %v", ErrLockInvalid, err)
	}
	lk := LockBinding{
		Name:   fields["name"],
		Alias:  fields["alias"],
		Commit: commit,
		Hash:   fields["hash"],
		URL:    fields["url"],
	}
	_, hasSource := fields["source"]
	_, hasResolution := fields["resolution"]
	if !hasSource && !hasResolution {
		// Selected-only: the version field is the raw request the resolver
		// selected. The strict decoder validates it as a request in
		// LockState.Validate; no canonical form is implied.
		lk.Requested = fields["version"]
		return lk, nil
	}
	if !hasSource || !hasResolution {
		return LockBinding{}, fmt.Errorf("%w: Lock %q must reference both a source and a resolution", ErrLockInvalid, lk.Key())
	}
	version, err := ParseCanonicalVersion(fields["version"])
	if err != nil {
		return LockBinding{}, fmt.Errorf("%w: Lock version: %v", ErrLockInvalid, err)
	}
	lk.SourceID = fields["source"]
	lk.ResolutionID = fields["resolution"]
	lk.Version = version
	return lk, nil
}

func encodeLockFormat(state LockState) ([]byte, error) {
	if state.Format != LockFormat1 {
		return nil, fmt.Errorf("%w: %d", ErrLockUnsupportedFormat, state.Format)
	}
	if err := state.Validate(); err != nil {
		return nil, err
	}
	state = state.Clone()
	state.SortRecords()
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s { format: %q, resolver: %q, inputs: %q }\n", state.Kind, strconv.Itoa(state.Format), state.Resolver, state.Inputs)
	for _, src := range state.Sources {
		sb.WriteString("\n")
		writeRecord(&sb, "Source",
			"id", src.ID,
			"name", src.Identity.Name,
			"url", src.Identity.URL,
			"subdir", src.Identity.Subdir)
	}
	for _, res := range state.Resolutions {
		sb.WriteString("\n")
		kv := []string{
			"id", res.ID,
			"source", res.SourceID,
			"requested", res.Requested,
			"version", res.Version.String(),
			"commit", res.Commit.String(),
			"time", FormatLockTime(res.Time),
			"manifest", res.Manifest,
			"tag", res.Tag,
		}
		if res.Base != nil {
			kv = append(kv,
				"baseVersion", res.Base.Version.String(),
				"baseTag", res.Base.Tag,
				"baseCommit", res.Base.Commit.String())
		}
		writeRecord(&sb, "Resolution", kv...)
	}
	for _, lk := range state.Locks {
		sb.WriteString("\n")
		kv := []string{"name", lk.Name, "alias", lk.Alias}
		if lk.Requested != "" {
			kv = append(kv, "version", lk.Requested)
		} else {
			kv = append(kv,
				"source", lk.SourceID,
				"resolution", lk.ResolutionID,
				"version", lk.Version.String())
		}
		kv = append(kv,
			"commit", lk.Commit.String(),
			"hash", lk.Hash,
			"url", lk.URL)
		writeRecord(&sb, "Lock", kv...)
	}
	return []byte(sb.String()), nil
}
