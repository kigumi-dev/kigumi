package module_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/module"
)

func lockDigest(ch byte) string { return "h1:" + strings.Repeat(string(ch), 64) }

func lockCommit(ch byte) string { return "sha1:" + strings.Repeat(string(ch), 40) }

func mustCanonical(t *testing.T, s string) module.CanonicalVersion {
	t.Helper()
	v, err := module.ParseCanonicalVersion(s)
	if err != nil {
		t.Fatalf("ParseCanonicalVersion(%q): %v", s, err)
	}
	return v
}

func mustCommit(t *testing.T, s string) module.CommitID {
	t.Helper()
	c, err := module.ParseCommitID(s)
	if err != nil {
		t.Fatalf("ParseCommitID(%q): %v", s, err)
	}
	return c
}

func lockTime(t *testing.T) time.Time {
	t.Helper()
	tm, err := module.ParseLockTime("2026-11-23T01:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

// sampleLockState is a complete first-format state with one winner, one loser
// request and an aliased binding.
func sampleLockState(t *testing.T, kind module.LockKind) module.LockState {
	t.Helper()
	identity, err := module.NewSourceIdentity("example.com/lib/a", "https://example.com/lib/a", "")
	if err != nil {
		t.Fatal(err)
	}
	sourceID := module.LockSourceID(identity)
	full := lockCommit('a')
	commit := mustCommit(t, full)
	version := mustCanonical(t, "v1.2.3")
	tm := lockTime(t)
	winnerID := module.LockResolutionID(sourceID, "v1.2.3")
	loserID := module.LockResolutionID(sourceID, "aaaaaaa")
	return module.LockState{
		Kind:     kind,
		Format:   module.LockFormat1,
		Resolver: module.LockResolverOrderedV1,
		Inputs:   lockDigest('1'),
		Sources: []module.LockSource{{
			ID:       sourceID,
			Identity: identity,
		}},
		Resolutions: []module.LockResolution{
			{
				ID:        winnerID,
				SourceID:  sourceID,
				Requested: "v1.2.3",
				Version:   version,
				Commit:    commit,
				Time:      tm,
				Manifest:  lockDigest('2'),
				Tag:       "v1.2.3",
			},
			{
				ID:        loserID,
				SourceID:  sourceID,
				Requested: "aaaaaaa",
				Version:   version,
				Commit:    commit,
				Time:      tm,
				Manifest:  lockDigest('2'),
			},
		},
		Locks: []module.LockBinding{
			{
				Name:         identity.Name,
				SourceID:     sourceID,
				ResolutionID: winnerID,
				Version:      version,
				Commit:       commit,
				Hash:         lockDigest('3'),
				URL:          identity.URL,
			},
			{
				Name:         identity.Name,
				Alias:        "libalias",
				SourceID:     sourceID,
				ResolutionID: loserID,
				Version:      version,
				Commit:       commit,
				Hash:         lockDigest('3'),
				URL:          identity.URL,
			},
		},
	}
}

// renderLockState writes records without validation, so tests can present a
// syntactically readable but semantically bad file to the strict decoder.
func renderLockState(st module.LockState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s { format: %q, resolver: %q, inputs: %q }\n", st.Kind, strconv.Itoa(st.Format), st.Resolver, st.Inputs)
	for _, src := range st.Sources {
		fmt.Fprintf(&b, "Source { id: %q, name: %q, url: %q, subdir: %q }\n",
			src.ID, src.Identity.Name, src.Identity.URL, src.Identity.Subdir)
	}
	for _, res := range st.Resolutions {
		fmt.Fprintf(&b, "Resolution { id: %q, source: %q, requested: %q, version: %q, commit: %q, time: %q, manifest: %q, tag: %q",
			res.ID, res.SourceID, res.Requested, res.Version.String(), res.Commit.String(), module.FormatLockTime(res.Time), res.Manifest, res.Tag)
		if res.Base != nil {
			fmt.Fprintf(&b, ", baseVersion: %q, baseTag: %q, baseCommit: %q",
				res.Base.Version.String(), res.Base.Tag, res.Base.Commit.String())
		}
		b.WriteString(" }\n")
	}
	for _, lk := range st.Locks {
		fmt.Fprintf(&b, "Lock { name: %q, alias: %q, source: %q, resolution: %q, version: %q, commit: %q, hash: %q, url: %q }\n",
			lk.Name, lk.Alias, lk.SourceID, lk.ResolutionID, lk.SelectedVersion(), lk.Commit.String(), lk.Hash, lk.URL)
	}
	return b.String()
}

func TestLockFormatRoundTripDeterministic(t *testing.T) {
	state := sampleLockState(t, module.LockKindModule)
	first, err := module.EncodeLock(state)
	if err != nil {
		t.Fatalf("EncodeLock: %v", err)
	}
	if !strings.HasPrefix(string(first), `ModuleLock { format: "1", resolver: "ordered-v1", inputs: "h1:`) {
		t.Fatalf("header is not the expected first line:\n%s", first)
	}
	decoded, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, first)
	if err != nil {
		t.Fatalf("DecodeLock: %v", err)
	}
	if err := module.ValidatePreservedBindings(state, decoded); err != nil {
		t.Fatalf("roundtrip lost meaning: %v", err)
	}
	second, err := module.EncodeLock(decoded)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("encoding is not byte-stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	shuffled := state.Clone()
	for i, j := 0, len(shuffled.Sources)-1; i < j; i, j = i+1, j-1 {
		shuffled.Sources[i], shuffled.Sources[j] = shuffled.Sources[j], shuffled.Sources[i]
	}
	for i, j := 0, len(shuffled.Resolutions)-1; i < j; i, j = i+1, j-1 {
		shuffled.Resolutions[i], shuffled.Resolutions[j] = shuffled.Resolutions[j], shuffled.Resolutions[i]
	}
	for i, j := 0, len(shuffled.Locks)-1; i < j; i, j = i+1, j-1 {
		shuffled.Locks[i], shuffled.Locks[j] = shuffled.Locks[j], shuffled.Locks[i]
	}
	shuffledBytes, err := module.EncodeLock(shuffled)
	if err != nil {
		t.Fatalf("EncodeLock(shuffled): %v", err)
	}
	if string(first) != string(shuffledBytes) {
		t.Fatal("record order changed the canonical encoding")
	}
}

func TestSelectedOnlyLockRoundTrip(t *testing.T) {
	state := module.EmptyLockState(module.LockKindModule)
	state.Inputs = lockDigest('1')
	state.Locks = []module.LockBinding{
		{
			Name:      "example.com/lib/a",
			Requested: "v1.2.3",
			Commit:    mustCommit(t, lockCommit('a')),
			Hash:      lockDigest('3'),
			URL:       "https://example.com/lib/a",
		},
		{
			Name:      "example.com/lib/a",
			Alias:     "libalias",
			Requested: "aaaaaaa",
			Commit:    mustCommit(t, lockCommit('a')),
			Hash:      lockDigest('3'),
		},
	}
	src, err := module.EncodeLock(state)
	if err != nil {
		t.Fatalf("EncodeLock(selected-only): %v", err)
	}
	if strings.Contains(string(src), "source:") || strings.Contains(string(src), "resolution:") {
		t.Fatalf("selected-only lock invented reference records:\n%s", src)
	}
	decoded, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, src)
	if err != nil {
		t.Fatalf("DecodeLock(selected-only): %v", err)
	}
	view := decoded.LockView()
	if got := view.Entries["example.com/lib/a"]; got.Version != "v1.2.3" || got.Name != "example.com/lib/a" {
		t.Fatalf("named entry = %+v", got)
	}
	if got := view.Entries["libalias"]; got.Version != "aaaaaaa" || got.Alias != "libalias" {
		t.Fatalf("aliased entry = %+v", got)
	}
	again, err := module.EncodeLock(decoded)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if string(src) != string(again) {
		t.Fatalf("selected-only encoding is not byte-stable:\n%s\n%s", src, again)
	}
}

func TestSelectedOnlyLockRejectsInconsistentEvidence(t *testing.T) {
	base := module.EmptyLockState(module.LockKindModule)
	base.Inputs = lockDigest('1')
	base.Locks = []module.LockBinding{{
		Name:      "example.com/lib/a",
		Requested: "v1.2.3",
		Commit:    mustCommit(t, lockCommit('a')),
		Hash:      lockDigest('3'),
	}}
	cases := []struct {
		name string
		edit func(*module.LockState)
	}{
		{"canonical version beside raw request", func(s *module.LockState) {
			s.Locks[0].Version = mustCanonical(t, "v1.2.3")
		}},
		{"missing request", func(s *module.LockState) {
			s.Locks[0].Requested = ""
		}},
		{"commit request is not a commit prefix", func(s *module.LockState) {
			s.Locks[0].Requested = "bbbbbbb"
		}},
		{"malformed request", func(s *module.LockState) {
			s.Locks[0].Requested = "not a version"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := base.Clone()
			tc.edit(&state)
			if err := state.Validate(); err == nil {
				t.Fatal("validated a selected-only lock with inconsistent evidence")
			}
			if _, err := module.EncodeLock(state); err == nil {
				t.Fatal("encoded a selected-only lock with inconsistent evidence")
			}
		})
	}
	t.Run("missing version field", func(t *testing.T) {
		src := `ModuleLock { format: "1", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n" +
			`Lock { name: "example.com/lib/a", commit: "` + lockCommit('a') + `", hash: "` + lockDigest('3') + `" }` + "\n"
		if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(src)); err == nil {
			t.Fatal("decoded a selected-only lock without a version")
		}
	})
	t.Run("empty request", func(t *testing.T) {
		src := `ModuleLock { format: "1", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n" +
			`Lock { name: "example.com/lib/a", version: "", commit: "` + lockCommit('a') + `", hash: "` + lockDigest('3') + `" }` + "\n"
		if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(src)); err == nil {
			t.Fatal("decoded a selected-only lock with an empty request")
		}
	})
}

func TestLockInputsDigestIsStableAndDistinguishesInputs(t *testing.T) {
	manifest := []byte("Module { name: \"app\" }\n")
	other := []byte("Module { name: \"app\" }\nRequire { name: \"example.com/x\", version: \"v1.0.0\" }\n")
	local := []byte("Replace { name: \"example.com/x\", path: \"../x\" }\n")
	first := module.LockInputsDigest(module.LockResolverOrderedV1, [][]byte{manifest}, nil)
	if again := module.LockInputsDigest(module.LockResolverOrderedV1, [][]byte{manifest}, nil); again != first {
		t.Fatalf("digest is not deterministic: %s != %s", first, again)
	}
	if module.LockInputsDigest(module.LockResolverOrderedV1, [][]byte{other}, nil) == first {
		t.Fatal("different manifest bytes produced the same inputs digest")
	}
	if module.LockInputsDigest(module.LockResolverOrderedV1, [][]byte{manifest}, local) == first {
		t.Fatal("local declarations did not change the inputs digest")
	}
	if module.LockInputsDigest(module.LockResolverOrderedV1, [][]byte{manifest}, nil) ==
		module.LockInputsDigest(module.LockResolverOrderedV1, [][]byte{manifest}, []byte{}) {
		t.Fatal("an absent local file and an empty one produced the same digest")
	}
	if !strings.HasPrefix(first, "h1:") {
		t.Fatalf("digest scheme = %q", first)
	}
}

func TestLockFormatEmptyGraphHeader(t *testing.T) {
	state := module.LockState{
		Kind:     module.LockKindModule,
		Format:   module.LockFormat1,
		Resolver: module.LockResolverOrderedV1,
		Inputs:   lockDigest('1'),
	}
	bytes, err := module.EncodeLock(state)
	if err != nil {
		t.Fatalf("EncodeLock(empty): %v", err)
	}
	decoded, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, bytes)
	if err != nil {
		t.Fatalf("DecodeLock(empty): %v", err)
	}
	if len(decoded.Sources)+len(decoded.Resolutions)+len(decoded.Locks) != 0 {
		t.Fatalf("empty header decoded records: %+v", decoded)
	}
}

func TestLockHeaderMalformed(t *testing.T) {
	validHeader := `ModuleLock { format: "1", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n"
	oldLock := `Lock { name: "example.com/x", version: "v1.0.0", commit: "` + lockCommit('a') + `", hash: "` + lockDigest('3') + `", url: "https://example.com/x" }` + "\n"
	cases := []struct {
		name string
		kind module.LockKind
		src  string
		want error
	}{
		{"empty", module.LockKindModule, "", module.ErrLockEmpty},
		{"whitespace", module.LockKindModule, " \n\t", module.ErrLockEmpty},
		{"headerless old lock", module.LockKindModule, oldLock, module.ErrLockMissingHeader},
		{"source before header", module.LockKindModule, `Source { id: "` + strings.Repeat("a", 64) + `", name: "example.com/x", url: "https://example.com/x" }` + "\n" + validHeader, module.ErrLockMissingHeader},
		{"duplicate header", module.LockKindModule, validHeader + validHeader, module.ErrLockInvalid},
		{"leading zero format", module.LockKindModule, `ModuleLock { format: "02", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n", module.ErrLockInvalid},
		{"negative format", module.LockKindModule, `ModuleLock { format: "-1", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n", module.ErrLockInvalid},
		{"fractional format", module.LockKindModule, `ModuleLock { format: "1.0", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n", module.ErrLockInvalid},
		{"overflow format", module.LockKindModule, `ModuleLock { format: "999999999999999999999999999", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n", module.ErrLockInvalid},
		{"future format", module.LockKindModule, `ModuleLock { format: "3", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n", module.ErrLockUnsupportedFormat},
		{"unpublished format", module.LockKindModule, `ModuleLock { format: "2", resolver: "ordered-v1", inputs: "` + lockDigest('1') + `" }` + "\n", module.ErrLockUnsupportedFormat},
		{"unknown resolver", module.LockKindModule, `ModuleLock { format: "1", resolver: "ordered-next", inputs: "` + lockDigest('1') + `" }` + "\n", module.ErrLockUnknownResolver},
		{"bad inputs", module.LockKindModule, `ModuleLock { format: "1", resolver: "ordered-v1", inputs: "sha256:abc" }` + "\n", module.ErrLockInvalid},
		{"kind mismatch", module.LockKindWorkspace, validHeader, module.ErrLockKindMismatch},
		{"unknown record", module.LockKindModule, validHeader + `Bogus { x: "y" }` + "\n", module.ErrLockInvalid},
		{"unknown field", module.LockKindModule, validHeader + `Source { id: "` + strings.Repeat("a", 64) + `", name: "example.com/x", url: "https://example.com/x", extra: "y" }` + "\n", module.ErrLockInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := module.DecodeLock("mod.lock.kg", tc.kind, []byte(tc.src))
			if err == nil {
				t.Fatal("decoded a malformed lock")
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want errors.Is %v", err, tc.want)
			}
		})
	}
}

func TestLockRecordValidation(t *testing.T) {
	base := sampleLockState(t, module.LockKindModule)
	cases := []struct {
		name string
		edit func(*module.LockState)
	}{
		{"bad commit width", func(s *module.LockState) {
			s.Resolutions[0].Commit = module.CommitID{Algorithm: module.CommitSHA1, Hex: strings.Repeat("a", 39)}
		}},
		{"bad manifest digest", func(s *module.LockState) {
			s.Resolutions[0].Manifest = "sha256:" + strings.Repeat("a", 64)
		}},
		{"bad lock hash", func(s *module.LockState) {
			s.Locks[0].Hash = "h1:short"
		}},
		{"missing source", func(s *module.LockState) {
			s.Resolutions[0].SourceID = strings.Repeat("0", 64)
		}},
		{"missing resolution", func(s *module.LockState) {
			s.Locks[0].ResolutionID = strings.Repeat("0", 64)
		}},
		{"duplicate source", func(s *module.LockState) {
			s.Sources = append(s.Sources, s.Sources[0])
		}},
		{"duplicate lock key", func(s *module.LockState) {
			s.Locks[1].Alias = ""
		}},
		{"lock version mismatch", func(s *module.LockState) {
			s.Locks[0].Version = mustCanonical(t, "v1.2.4")
		}},
		{"lock commit mismatch", func(s *module.LockState) {
			s.Locks[0].Commit = mustCommit(t, lockCommit('b'))
		}},
		{"duplicate resolution", func(s *module.LockState) {
			s.Resolutions[1].Requested = s.Resolutions[0].Requested
			s.Resolutions[1].ID = s.Resolutions[0].ID
		}},
		{"same version different commit", func(s *module.LockState) {
			s.Resolutions[1].Requested = "bbbbbbb"
			s.Resolutions[1].Commit = mustCommit(t, lockCommit('b'))
			s.Resolutions[1].ID = module.LockResolutionID(s.Resolutions[1].SourceID, "bbbbbbb")
		}},
		{"raw hash not a commit prefix", func(s *module.LockState) {
			s.Resolutions[1].Requested = "bbbbbbb"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := base.Clone()
			tc.edit(&state)
			_, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(renderLockState(state)))
			if err == nil {
				t.Fatal("decoded an invalid lock")
			}
		})
	}
	t.Run("bad time offset", func(t *testing.T) {
		src := strings.Replace(renderLockState(base), "2026-11-23T01:00:00Z", "2026-11-23T01:00:00+01:00", 1)
		if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(src)); err == nil {
			t.Fatal("accepted a non-UTC lock time")
		}
	})
}

func TestLockCommitRequiresAlgorithm(t *testing.T) {
	base := sampleLockState(t, module.LockKindModule)
	src := strings.Replace(renderLockState(base), `commit: "`+lockCommit('a')+`"`, `commit: "`+strings.Repeat("a", 40)+`"`, 1)
	if src == renderLockState(base) {
		t.Fatal("test source was not rewritten")
	}
	if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(src)); err == nil {
		t.Fatal("accepted a bare commit id without a sha1:/sha256: prefix")
	}
}

func TestLockPseudoVersionValidation(t *testing.T) {
	identity, err := module.NewSourceIdentity("example.com/lib/a", "https://example.com/lib/a", "")
	if err != nil {
		t.Fatal(err)
	}
	sourceID := module.LockSourceID(identity)
	tm := lockTime(t)
	base := mustCanonical(t, "v1.2.3")
	baseCommit := mustCommit(t, lockCommit('b'))
	commit := mustCommit(t, lockCommit('c'))
	pseudo, err := module.NewPseudoVersion(identity.Name, &base, tm, commit)
	if err != nil {
		t.Fatalf("NewPseudoVersion: %v", err)
	}
	state := module.LockState{
		Kind:     module.LockKindModule,
		Format:   module.LockFormat1,
		Resolver: module.LockResolverOrderedV1,
		Inputs:   lockDigest('1'),
		Sources:  []module.LockSource{{ID: sourceID, Identity: identity}},
		Resolutions: []module.LockResolution{{
			ID:        module.LockResolutionID(sourceID, pseudo.String()),
			SourceID:  sourceID,
			Requested: pseudo.String(),
			Version:   pseudo,
			Commit:    commit,
			Time:      tm,
			Manifest:  lockDigest('2'),
			Base: &module.RevisionBase{
				Version: base,
				Tag:     "v1.2.3",
				Commit:  baseCommit,
			},
		}},
		Locks: []module.LockBinding{{
			Name:         identity.Name,
			SourceID:     sourceID,
			ResolutionID: module.LockResolutionID(sourceID, pseudo.String()),
			Version:      pseudo,
			Commit:       commit,
			Hash:         lockDigest('3'),
			URL:          identity.URL,
		}},
	}
	if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(renderLockState(state))); err != nil {
		t.Fatalf("valid pseudo-resolution rejected: %v", err)
	}

	t.Run("wrong time", func(t *testing.T) {
		bad := state.Clone()
		bad.Resolutions[0].Time = tm.Add(time.Second)
		if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(renderLockState(bad))); err == nil {
			t.Fatal("accepted a pseudo-version whose time differs from the commit")
		}
	})
	t.Run("missing base", func(t *testing.T) {
		bad := state.Clone()
		bad.Resolutions[0].Base = nil
		if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(renderLockState(bad))); err == nil {
			t.Fatal("accepted a based pseudo-version without base evidence")
		}
	})
	t.Run("wrong base version", func(t *testing.T) {
		bad := state.Clone()
		wrong := mustCanonical(t, "v1.2.2")
		bad.Resolutions[0].Base = &module.RevisionBase{Version: wrong, Tag: "v1.2.2", Commit: baseCommit}
		if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(renderLockState(bad))); err == nil {
			t.Fatal("accepted a pseudo-version with the wrong base")
		}
	})
}

func TestLockFileStates(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "mod.lock.kg")
	if _, err := module.ReadLockStateFile(missing, module.LockKindModule); !errors.Is(err, module.ErrLockMissing) {
		t.Fatalf("missing file error = %v", err)
	}
	if err := os.WriteFile(missing, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := module.ReadLockStateFile(missing, module.LockKindModule); !errors.Is(err, module.ErrLockEmpty) {
		t.Fatalf("empty file error = %v", err)
	}
	if _, err := module.ReadLockFile(missing); !errors.Is(err, module.ErrLockEmpty) {
		t.Fatalf("empty file via ReadLockFile error = %v, want ErrLockEmpty", err)
	}
	empty := module.LockState{
		Kind:     module.LockKindModule,
		Format:   module.LockFormat1,
		Resolver: module.LockResolverOrderedV1,
		Inputs:   lockDigest('1'),
	}
	bytes, err := module.EncodeLock(empty)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(missing, bytes, 0o644); err != nil {
		t.Fatal(err)
	}
	decoded, err := module.ReadLockStateFile(missing, module.LockKindModule)
	if err != nil {
		t.Fatalf("zero-dependency header: %v", err)
	}
	if len(decoded.Locks) != 0 {
		t.Fatalf("zero-dependency lock has %d bindings", len(decoded.Locks))
	}
}

func TestLockCodecNeverFallsBack(t *testing.T) {
	empty := module.LockState{
		Kind:     module.LockKindModule,
		Format:   module.LockFormat1,
		Resolver: module.LockResolverOrderedV1,
		Inputs:   lockDigest('1'),
	}
	src, err := module.EncodeLock(empty)
	if err != nil {
		t.Fatal(err)
	}
	codecs := module.NewLockCodecs()
	if err := codecs.Register(module.LockFormatCodec{
		Format: 3,
		Decode: func(string, module.LockKind, module.LockHeader, []byte) (module.LockState, error) {
			return module.LockState{}, nil
		},
		Encode: func(module.LockState) ([]byte, error) { return nil, nil },
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := codecs.Decode("mod.lock.kg", module.LockKindModule, src); !errors.Is(err, module.ErrLockUnsupportedFormat) {
		t.Fatalf("the published format read by another decoder: %v", err)
	}
}

func TestLockHeaderlessRejected(t *testing.T) {
	// A lock written before the published format has no decoder: ReadLock must
	// fail on it instead of treating it as an empty or older lock.
	path := filepath.Join(t.TempDir(), "mod.lock.kg")
	src := `Lock { name: "example.com/x", version: "v1.0.0", commit: "` + lockCommit('a') + `", hash: "` + lockDigest('3') + `", url: "https://example.com/x" }` + "\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := module.ReadLockFile(path); !errors.Is(err, module.ErrLockMissingHeader) {
		t.Fatalf("headerless lock error = %v, want ErrLockMissingHeader", err)
	}
	if _, err := module.ReadLock(filepath.Dir(path)); !errors.Is(err, module.ErrLockMissingHeader) {
		t.Fatalf("headerless lock via ReadLock error = %v, want ErrLockMissingHeader", err)
	}
}
