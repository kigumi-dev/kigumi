package module_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

func recomputeLockIDs(s *module.LockState) {
	sourceIDs := map[string]string{}
	for i := range s.Sources {
		next := module.LockSourceID(s.Sources[i].Identity)
		sourceIDs[s.Sources[i].ID] = next
		s.Sources[i].ID = next
	}
	resolutionIDs := map[string]string{}
	for i := range s.Resolutions {
		s.Resolutions[i].SourceID = sourceIDs[s.Resolutions[i].SourceID]
		next := module.LockResolutionID(s.Resolutions[i].SourceID, s.Resolutions[i].Requested)
		resolutionIDs[s.Resolutions[i].ID] = next
		s.Resolutions[i].ID = next
	}
	for i := range s.Locks {
		s.Locks[i].SourceID = sourceIDs[s.Locks[i].SourceID]
		s.Locks[i].ResolutionID = resolutionIDs[s.Locks[i].ResolutionID]
	}
}

func syntheticLockID(group byte, i int) string {
	return strings.Repeat(string(group), 60) + fmt.Sprintf("%04x", i+1)
}

func lockByKey(s module.LockState, key string) module.LockBinding {
	for _, lk := range s.Locks {
		if lk.Key() == key {
			return lk
		}
	}
	return module.LockBinding{}
}

func sourceByID(s module.LockState, id string) module.LockSource {
	for _, src := range s.Sources {
		if src.ID == id {
			return src
		}
	}
	return module.LockSource{}
}

// fixtureRegistry registers a test-only 1 -> 2 -> 3 chain. The synthetic
// second format drops the redundant Lock.url field and the third restores it;
// both re-derive every record ID. No command ever sees these formats.
func fixtureRegistry(t *testing.T, kind module.LockKind, recomputes *int) *module.LockUpgradeRegistry {
	t.Helper()
	r := module.NewLockUpgradeRegistry()
	addNext := module.LockUpgradeStep{
		Kind:  kind,
		From:  module.LockFormat1,
		To:    module.LockFormat1 + 1,
		ID:    "drop-url",
		Class: module.LockUpgradePure,
		Transform: func(in module.LockState, _ []module.PinnedEvidence) (module.LockState, error) {
			out := in.Clone()
			out.Format = module.LockFormat1 + 1
			for i := range out.Locks {
				out.Locks[i].URL = ""
			}
			// The synthetic format uses a different record-ID scheme.
			// Preservation must follow references, not compare ID strings.
			sourceIDs := map[string]string{}
			for i := range out.Sources {
				next := syntheticLockID('3', i)
				sourceIDs[out.Sources[i].ID] = next
				out.Sources[i].ID = next
			}
			resolutionIDs := map[string]string{}
			for i := range out.Resolutions {
				out.Resolutions[i].SourceID = sourceIDs[out.Resolutions[i].SourceID]
				next := syntheticLockID('4', i)
				resolutionIDs[out.Resolutions[i].ID] = next
				out.Resolutions[i].ID = next
			}
			for i := range out.Locks {
				out.Locks[i].SourceID = sourceIDs[out.Locks[i].SourceID]
				out.Locks[i].ResolutionID = resolutionIDs[out.Locks[i].ResolutionID]
			}
			if recomputes != nil {
				(*recomputes)++
			}
			return out, nil
		},
	}
	addFinal := module.LockUpgradeStep{
		Kind:  kind,
		From:  module.LockFormat1 + 1,
		To:    module.LockFormat1 + 2,
		ID:    "restore-url",
		Class: module.LockUpgradePure,
		Transform: func(in module.LockState, _ []module.PinnedEvidence) (module.LockState, error) {
			out := in.Clone()
			out.Format = module.LockFormat1 + 2
			sources := map[string]string{}
			for _, src := range out.Sources {
				sources[src.ID] = src.Identity.URL
			}
			for i := range out.Locks {
				out.Locks[i].URL = sources[out.Locks[i].SourceID]
			}
			recomputeLockIDs(&out)
			if recomputes != nil {
				(*recomputes)++
			}
			return out, nil
		},
	}
	for _, step := range []module.LockUpgradeStep{addNext, addFinal} {
		if err := r.Register(step); err != nil {
			t.Fatalf("Register(%s): %v", step.ID, err)
		}
	}
	return r
}

func TestUpgradePureChainPreservesMeaning(t *testing.T) {
	state := sampleLockState(t, module.LockKindModule)
	original := state.Clone()
	encoded, err := module.EncodeLock(state)
	if err != nil {
		t.Fatal(err)
	}
	recomputes := 0
	reg := fixtureRegistry(t, module.LockKindModule, &recomputes)
	plan, err := reg.Plan(state, 3, encoded)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.SourceDigest != module.LockBytesDigest(encoded) {
		t.Fatalf("plan source digest = %q", plan.SourceDigest)
	}
	if len(plan.Steps) != 2 || plan.Steps[0] != "drop-url" || plan.Steps[1] != "restore-url" {
		t.Fatalf("plan steps = %v", plan.Steps)
	}
	out, report, err := reg.Upgrade(state, 3, encoded)
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if out.Format != 3 {
		t.Fatalf("format = %d, want 4", out.Format)
	}
	if recomputes != 6 {
		t.Fatalf("ID reconstruction ran %d times, want 6 (explicit Plan, Upgrade plan, Apply; two steps each)", recomputes)
	}
	if err := module.ValidatePreservedBindings(original, out); err != nil {
		t.Fatalf("pure chain lost meaning: %v", err)
	}
	if renderLockState(state) != renderLockState(original) {
		t.Fatal("upgrade mutated the input state")
	}
	if got := lockByKey(out, "libalias").URL; got != "https://example.com/lib/a" {
		t.Fatalf("alias URL after 3->4 = %q", got)
	}
	if report.ResolutionChanged || report.MetadataAdded {
		t.Fatalf("report = %+v, want representation-only", report)
	}
	if report.WriteFormat != 3 || report.From != 1 || report.To != 3 {
		t.Fatalf("report = %+v", report)
	}

	// Re-running the latest format is a no-op and produces the same bytes
	// under the test format-4 representation.
	again, againReport, err := reg.Upgrade(out, 3, nil)
	if err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if len(againReport.Steps) != 0 {
		t.Fatalf("re-run steps = %v, want none", againReport.Steps)
	}
	if renderLockState(out) != renderLockState(again) {
		t.Fatal("re-running the latest format changed the state")
	}
}

func TestUpgradeRegistryRejectsBadPaths(t *testing.T) {
	pure := func(id string, from, to int) module.LockUpgradeStep {
		return module.LockUpgradeStep{
			Kind: module.LockKindModule, From: from, To: to, ID: id, Class: module.LockUpgradePure,
			Transform: func(in module.LockState, _ []module.PinnedEvidence) (module.LockState, error) {
				out := in.Clone()
				out.Format = to
				return out, nil
			},
		}
	}
	t.Run("skip", func(t *testing.T) {
		r := module.NewLockUpgradeRegistry()
		if err := r.Register(pure("skip", 1, 3)); err == nil {
			t.Fatal("registered a format skip")
		}
	})
	t.Run("cycle or downgrade", func(t *testing.T) {
		r := module.NewLockUpgradeRegistry()
		if err := r.Register(pure("cycle", 3, 2)); err == nil {
			t.Fatal("registered a backward step")
		}
	})
	t.Run("duplicate edge", func(t *testing.T) {
		r := module.NewLockUpgradeRegistry()
		if err := r.Register(pure("a", 2, 3)); err != nil {
			t.Fatal(err)
		}
		if err := r.Register(pure("b", 2, 3)); !errors.Is(err, module.ErrLockUpgradeDuplicateStep) {
			t.Fatalf("duplicate edge error = %v", err)
		}
	})
	t.Run("duplicate id", func(t *testing.T) {
		r := module.NewLockUpgradeRegistry()
		if err := r.Register(pure("same", 2, 3)); err != nil {
			t.Fatal(err)
		}
		if err := r.Register(pure("same", 2, 3)); !errors.Is(err, module.ErrLockUpgradeDuplicateStep) {
			t.Fatalf("duplicate id error = %v", err)
		}
	})
	t.Run("missing step", func(t *testing.T) {
		r := module.NewLockUpgradeRegistry()
		if err := r.Register(pure("only", 2, 3)); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Plan(sampleLockState(t, module.LockKindModule), 3, nil); !errors.Is(err, module.ErrLockUpgradeMissingStep) {
			t.Fatalf("missing step error = %v", err)
		}
	})
	t.Run("downgrade", func(t *testing.T) {
		r := fixtureRegistry(t, module.LockKindModule, nil)
		state := sampleLockState(t, module.LockKindModule)
		state.Format = 3
		if _, err := r.Plan(state, 2, nil); !errors.Is(err, module.ErrLockUpgradeMissingStep) {
			t.Fatalf("downgrade error = %v", err)
		}
	})
}

func TestUpgradeRejectsLostMeaning(t *testing.T) {
	withLoser := func() module.LockState {
		state := sampleLockState(t, module.LockKindModule)
		src := state.Sources[0]
		state.Edges = []module.LockEdge{{From: "root", To: "example.com/lib/a"}}
		state.Resolutions = append(state.Resolutions, module.LockResolution{
			ID:        module.LockResolutionID(src.ID, "aaaaaa1"),
			SourceID:  src.ID,
			Requested: "aaaaaa1",
			Version:   mustCanonical(t, "v1.2.3"),
			Commit:    mustCommit(t, lockCommit('a')),
			Time:      lockTime(t),
			Manifest:  lockDigest('2'),
		})
		return state
	}
	cases := []struct {
		name string
		edit func(*module.LockState)
	}{
		{"drop loser query", func(s *module.LockState) {
			s.Resolutions = s.Resolutions[:len(s.Resolutions)-1]
		}},
		{"rename alias", func(s *module.LockState) {
			s.Locks[1].Alias = "otheralias"
		}},
		{"change source", func(s *module.LockState) {
			identity, err := module.NewSourceIdentity(s.Sources[0].Identity.Name, "https://example.com/lib/other", "")
			if err != nil {
				t.Fatal(err)
			}
			s.Sources[0].Identity = identity
			for i := range s.Locks {
				s.Locks[i].URL = ""
			}
			recomputeLockIDs(s)
		}},
		{"change manifest digest", func(s *module.LockState) {
			s.Resolutions[0].Manifest = lockDigest('f')
		}},
		{"change canonical version", func(s *module.LockState) {
			version := mustCanonical(t, "v1.2.4")
			s.Resolutions[0].Version = version
			s.Locks[0].Version = version
		}},
		{"change full commit", func(s *module.LockState) {
			commit := mustCommit(t, lockCommit('b'))
			s.Resolutions[0].Commit = commit
			s.Locks[0].Commit = commit
		}},
		{"drop selected edge", func(s *module.LockState) {
			s.Edges = nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := withLoser()
			reg := module.NewLockUpgradeRegistry()
			err := reg.Register(module.LockUpgradeStep{
				Kind: module.LockKindModule, From: 1, To: 2, ID: "lossy", Class: module.LockUpgradePure,
				Transform: func(in module.LockState, _ []module.PinnedEvidence) (module.LockState, error) {
					out := in.Clone()
					out.Format = 2
					tc.edit(&out)
					recomputeLockIDs(&out)
					return out, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := reg.Upgrade(state, 3, nil); !errors.Is(err, module.ErrLockPreservation) {
				t.Fatalf("lossy step error = %v, want ErrLockPreservation", err)
			}
		})
	}
}

func TestUpgradeClassificationAndEvidence(t *testing.T) {
	t.Run("NeedsResolution", func(t *testing.T) {
		reg := module.NewLockUpgradeRegistry()
		if err := reg.Register(module.LockUpgradeStep{
			Kind: module.LockKindModule, From: 1, To: 2, ID: "reresolve",
			Class: module.LockUpgradeNeedsResolution, Reason: "canonical version scheme changes",
		}); err != nil {
			t.Fatal(err)
		}
		state := sampleLockState(t, module.LockKindModule)
		if _, err := reg.Plan(state, 3, nil); !errors.Is(err, module.ErrLockUpgradeNeedsResolution) {
			t.Fatalf("Plan error = %v", err)
		}
		if _, _, err := reg.Upgrade(state, 3, nil); !errors.Is(err, module.ErrLockUpgradeNeedsResolution) {
			t.Fatalf("Upgrade error = %v", err)
		}
	})

	t.Run("EnrichPinned", func(t *testing.T) {
		reg := module.NewLockUpgradeRegistry()
		err := reg.Register(module.LockUpgradeStep{
			Kind: module.LockKindModule, From: 1, To: 2, ID: "add-evidence",
			Class: module.LockUpgradeEnrichPinned,
			Evidence: func(in module.LockState) ([]module.PinnedEvidenceRequest, error) {
				res := in.Resolutions[0]
				return []module.PinnedEvidenceRequest{{
					SourceID: res.SourceID,
					Source:   in.Sources[0].Identity,
					Commit:   res.Commit,
					Needs:    []module.PinnedEvidenceKind{module.PinnedEvidenceManifest, module.PinnedEvidenceTime},
					Existing: module.LockPinnedEvidence{Manifest: res.Manifest, Time: res.Time},
				}}, nil
			},
			Transform: func(in module.LockState, evidence []module.PinnedEvidence) (module.LockState, error) {
				if len(evidence) != 1 {
					t.Fatalf("transform got %d evidence entries", len(evidence))
				}
				out := in.Clone()
				out.Format = 2
				return out, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		state := sampleLockState(t, module.LockKindModule)
		res := state.Resolutions[0]
		plan, err := reg.Plan(state, 3, nil)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		if !plan.NeedsEvidence || len(plan.Evidence) != 1 {
			t.Fatalf("plan = %+v", plan)
		}
		if _, _, err := reg.Upgrade(state, 3, nil); !errors.Is(err, module.ErrLockUpgradeNeedsEvidence) {
			t.Fatalf("Upgrade without evidence = %v", err)
		}
		out, err := reg.ApplyPlan(state, plan, []module.PinnedEvidence{{
			SourceID: res.SourceID,
			Commit:   res.Commit,
			Manifest: res.Manifest,
			Hash:     lockDigest('3'),
			Time:     res.Time,
		}})
		if err != nil {
			t.Fatalf("ApplyPlan: %v", err)
		}
		if out.Format != 2 {
			t.Fatalf("format = %d", out.Format)
		}
		bad := []module.PinnedEvidence{{
			SourceID: res.SourceID,
			Commit:   mustCommit(t, lockCommit('b')),
			Manifest: res.Manifest,
			Time:     res.Time,
		}}
		if _, err := reg.ApplyPlan(state, plan, bad); err == nil {
			t.Fatal("accepted evidence for a different commit")
		}
	})

	t.Run("unknown resolver is not a migration", func(t *testing.T) {
		src := `ModuleLock { format: "1", resolver: "ordered-next", inputs: "` + lockDigest('1') + `" }` + "\n"
		if _, err := module.DecodeLock("mod.lock.kg", module.LockKindModule, []byte(src)); !errors.Is(err, module.ErrLockUnknownResolver) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestUpgradeKindSeparation(t *testing.T) {
	moduleState := sampleLockState(t, module.LockKindModule)
	workspaceState := sampleLockState(t, module.LockKindWorkspace)
	moduleReg := fixtureRegistry(t, module.LockKindModule, nil)
	workspaceReg := fixtureRegistry(t, module.LockKindWorkspace, nil)

	moduleOut, moduleReport, err := moduleReg.Upgrade(moduleState, 3, nil)
	if err != nil {
		t.Fatalf("module upgrade: %v", err)
	}
	workspaceOut, workspaceReport, err := workspaceReg.Upgrade(workspaceState, 3, nil)
	if err != nil {
		t.Fatalf("workspace upgrade: %v", err)
	}
	if err := module.ValidatePreservedBindings(moduleState, moduleOut); err != nil {
		t.Fatal(err)
	}
	if err := module.ValidatePreservedBindings(workspaceState, workspaceOut); err != nil {
		t.Fatal(err)
	}
	if moduleReport.Steps[0] != workspaceReport.Steps[0] || moduleReport.Steps[1] != workspaceReport.Steps[1] {
		t.Fatalf("shared steps differ: %v vs %v", moduleReport.Steps, workspaceReport.Steps)
	}
	if moduleOut.Locks[0] != workspaceOut.Locks[0] || moduleOut.Resolutions[0] != workspaceOut.Resolutions[0] {
		t.Fatal("the same records did not migrate identically for module and workspace")
	}
	if moduleOut.Sources[0] != moduleState.Sources[0] || workspaceOut.Sources[0] != workspaceState.Sources[0] {
		t.Fatal("a pure format step changed its source record")
	}

	if _, ok := moduleReg.Step(module.LockKindWorkspace, 2); ok {
		t.Fatal("module registry answered for workspace")
	}
	if _, err := moduleReg.Plan(workspaceState, 3, nil); !errors.Is(err, module.ErrLockUpgradeMissingStep) {
		t.Fatalf("workspace plan on module registry = %v", err)
	}
	bytes, err := module.EncodeLock(moduleState)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.DecodeLock("workspace.lock.kg", module.LockKindWorkspace, bytes); !errors.Is(err, module.ErrLockKindMismatch) {
		t.Fatalf("kind mismatch = %v", err)
	}
}

func TestChooseLockWriteFormat(t *testing.T) {
	supports := []module.LockFormatSupport{
		{Format: 1, Resolver: module.LockResolverOrderedV1, MinToolchain: "0.1"},
		{Format: 2, Resolver: module.LockResolverOrderedV1, MinToolchain: "0.2"},
		{Format: 3, Resolver: module.LockResolverOrderedV1, MinToolchain: "0.3"},
	}
	cases := []struct {
		name          string
		currentFormat int
		resolver      string
		minToolchain  string
		want          int
		wantErr       bool
	}{
		{"old minimum keeps the published format", 1, module.LockResolverOrderedV1, "0.1", 1, false},
		{"new minimum raises writer format", 1, module.LockResolverOrderedV1, "0.3", 3, false},
		{"unspecified minimum keeps existing format", 1, module.LockResolverOrderedV1, "", 1, false},
		{"new lock with declared minimum", 0, module.LockResolverOrderedV1, "0.2", 2, false},
		{"new lock without minimum uses newest", 0, module.LockResolverOrderedV1, "", 3, false},
		{"declared minimum cannot downgrade", 3, module.LockResolverOrderedV1, "0.1", 0, true},
		{"minimum reads nothing", 1, module.LockResolverOrderedV1, "0.0", 0, true},
		{"unknown current resolver", 1, "ordered-next", "0.1", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := module.ChooseLockWriteFormat(tc.currentFormat, tc.resolver, tc.minToolchain, supports)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("chose format %d, want error", got.Format)
				}
				return
			}
			if err != nil {
				t.Fatalf("ChooseLockWriteFormat: %v", err)
			}
			if got.Format != tc.want {
				t.Fatalf("format = %d, want %d", got.Format, tc.want)
			}
		})
	}
	t.Run("duplicate support", func(t *testing.T) {
		dup := append([]module.LockFormatSupport(nil), supports...)
		dup = append(dup, dup[0])
		if _, err := module.ChooseLockWriteFormat(1, module.LockResolverOrderedV1, "0.1", dup); err == nil {
			t.Fatal("accepted duplicate format support")
		}
	})
	t.Run("unsupported resolver", func(t *testing.T) {
		bad := []module.LockFormatSupport{{Format: 1, Resolver: "ordered-next", MinToolchain: "0.1"}}
		if _, err := module.ChooseLockWriteFormat(1, "ordered-next", "0.1", bad); !errors.Is(err, module.ErrLockUnknownResolver) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("plan separates memory format from write format", func(t *testing.T) {
		state := sampleLockState(t, module.LockKindModule)
		reg := fixtureRegistry(t, module.LockKindModule, nil)
		out, report, err := reg.Upgrade(state, 3, nil)
		if err != nil {
			t.Fatal(err)
		}
		decision, err := module.ChooseLockWriteFormat(report.From, out.Resolver, "0.1", supports)
		if err != nil {
			t.Fatal(err)
		}
		if out.Format != 3 || decision.Format != 1 {
			t.Fatalf("memory format = %d, write format = %d", out.Format, decision.Format)
		}
		if report.ResolutionChanged {
			t.Fatal("format-only migration reported a dependency change")
		}
	})
}

func TestUpgradeDetectsInputMutation(t *testing.T) {
	reg := module.NewLockUpgradeRegistry()
	err := reg.Register(module.LockUpgradeStep{
		Kind: module.LockKindModule, From: 1, To: 2, ID: "mutating", Class: module.LockUpgradePure,
		Transform: func(in module.LockState, _ []module.PinnedEvidence) (module.LockState, error) {
			// Mutate a shared slice element; scalar fields of a by-value
			// parameter cannot reach the caller.
			in.Sources[0].Identity.Name = "example.com/mutated"
			in.Format = 3
			return in, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := sampleLockState(t, module.LockKindModule)
	if _, _, err := reg.Upgrade(state, 3, nil); err == nil || !strings.Contains(err.Error(), "mutated its input") {
		t.Fatalf("mutating step error = %v", err)
	}
}
