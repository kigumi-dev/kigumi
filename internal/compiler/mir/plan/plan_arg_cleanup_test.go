package plan_test

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/devkit/mirfixture"
)

// The default argument runs in a helper that only borrows the resource
// arguments. When it fails, those arguments never reach the callee's body, so
// the caller still owns them and must release them newest first.
const argCleanupSrc = `
type Item = resource { label String }
fn Item.drop(move self) { print "drop ${self.label}" }
type Problem = { }
fn Problem.message(self) -> String { "failed" }
fn make(label: String) -> Item { Item { label: label } }
fn stop() -> Int! { fail Problem {} }
fn ok() -> Int! { 7 }
fn collect(first: Item, third: Item, second: Int = stop()?) -> Void! { () }
fn collectOk(first: Item, third: Item, second: Int = ok()?) -> Void! { () }

fn defaults() -> Void! { collect(first: make("first"), third: make("third"))? }
fn named() -> Void! { collect(third: make("third"), first: make("first"))? }
fn explicit() -> Void! { collect(first: make("first"), third: make("third"), second: stop()?) }
fn success() -> Void! { collectOk(first: make("first"), third: make("third"))? }
`

func callResults(p *mir.Program, f *midir.Func, callee string, argc int) []midir.LocalID {
	var out []midir.LocalID
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != midir.OpCall || in.Ent == 0 || p.R.Entity(in.Ent).Name != callee {
				continue
			}
			// A default argument is evaluated in a synthesized callee that
			// carries the callee's name; only the explicit call has every
			// argument.
			if argc >= 0 && len(in.Args) != argc {
				continue
			}
			out = append(out, in.Dst)
		}
	}
	return out
}

type releaseRun struct {
	block midir.BlockID
	ids   []midir.LocalID
}

// releaseRuns are the maximal runs of releases naming one of members, in
// instruction order, so a test sees the order the backend will execute.
func releaseRuns(f *midir.Func, members map[midir.LocalID]bool) []releaseRun {
	var runs []releaseRun
	for b, blk := range f.Blocks {
		var cur []midir.LocalID
		flush := func() {
			if len(cur) > 0 {
				runs = append(runs, releaseRun{block: midir.BlockID(b), ids: cur})
				cur = nil
			}
		}
		for _, in := range blk.Insts {
			if in.Op == midir.OpRelease && len(in.Args) == 1 && members[in.Args[0]] {
				cur = append(cur, in.Args[0])
				continue
			}
			flush()
		}
		flush()
	}
	return runs
}

func TestArgumentCleanupReleasesInReverseCreationOrder(t *testing.T) {
	p := mirfixture.Planned(t, argCleanupSrc)
	for _, tc := range []struct {
		name   string
		callee string
	}{
		{"main.defaults", "collect"},
		{"main.named", "collect"},
		{"main.explicit", "collect"},
		{"main.success", "collectOk"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := mirfixture.FuncNamed(t, p, tc.name)
			made := callResults(p, f, "make", 1)
			if len(made) != 2 {
				t.Fatalf("%s: made %d resources, want 2:\n%s", tc.name, len(made), p.Dump())
			}
			if body := callResults(p, f, tc.callee, 3); len(body) != 1 {
				t.Fatalf("%s: the callee body call is missing (%d calls):\n%s", tc.name, len(body), p.Dump())
			}
			members := map[midir.LocalID]bool{made[0]: true, made[1]: true}
			// The second make ran later, so its resource goes first. The
			// default's failure block exists even when its body succeeds;
			// which branch runs is the job of the driver test.
			want := []midir.LocalID{made[1], made[0]}
			runs := releaseRuns(f, members)
			if len(runs) != 1 || !slices.Equal(runs[0].ids, want) {
				t.Fatalf("%s: release runs %v, want exactly %v (newest first):\n%s", tc.name, runs, want, p.Dump())
			}
			// The cleanup path returns the error rather than entering the body.
			if reaches(successors(f), runs[0].block, blockDefining(f, callResults(p, f, tc.callee, 3)[0])) {
				t.Fatalf("%s: the cleanup releases flow into the callee body:\n%s", tc.name, p.Dump())
			}
		})
	}
}

func successors(f *midir.Func) [][]midir.BlockID {
	out := make([][]midir.BlockID, len(f.Blocks))
	for b, blk := range f.Blocks {
		out[b] = append(out[b], blk.Term.Targets...)
	}
	return out
}

func reaches(succ [][]midir.BlockID, from, to midir.BlockID) bool {
	seen := map[midir.BlockID]bool{}
	stack := append([]midir.BlockID(nil), succ[from]...)
	for len(stack) > 0 {
		b := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if b == to {
			return true
		}
		if seen[b] {
			continue
		}
		seen[b] = true
		stack = append(stack, succ[b]...)
	}
	return false
}

func blockDefining(f *midir.Func, dst midir.LocalID) midir.BlockID {
	for b, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Dst == dst && in.Op.Yields() {
				return midir.BlockID(b)
			}
		}
	}
	return 0
}

func resourceReleaseBlocks(p *mir.Program, f *midir.Func) []midir.BlockID {
	var out []midir.BlockID
	for b, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpRelease && len(in.Args) == 1 {
				if l := f.Locals[in.Args[0]]; l.Ent == 0 && p.R.HoldsResource(l.Type) {
					out = append(out, midir.BlockID(b))
				}
			}
		}
	}
	return out
}

// A join and a loop must keep the same release order and drop each resource
// once: the plan decides the edge cleanup into the merge and the cleanup at
// the end of every iteration.
func TestArgumentCleanupSurvivesJoinAndLoop(t *testing.T) {
	p := mirfixture.Planned(t, argCleanupSrc+`
fn pair(a: &Item, b: &Item) { print "${a.label}/${b.label}" }

fn joined(flag: Bool) -> Void! {
    pair(&(if flag { make("a") } else { make("x") }), &(if flag { make("b") } else { make("y") }))
}

fn looped(times: Int) -> Void! {
    let mut i = 0
    for i < times {
        pair(&make("a"), &make("b"))
        i = i + 1
    }
}
`)
	for _, fname := range []string{"main.joined", "main.looped"} {
		t.Run(fname, func(t *testing.T) {
			f := mirfixture.FuncNamed(t, p, fname)
			released := releasedLocals(f)
			for id, n := range released {
				if n != 1 {
					t.Errorf("%s: %%%s is released %d times, want once:\n%s", fname, f.Locals[id].Name, n, p.Dump())
				}
			}
			blocks := resourceReleaseBlocks(p, f)
			if len(blocks) == 0 {
				t.Fatalf("%s: no resource temporary is released:\n%s", fname, p.Dump())
			}
			switch fname {
			case "main.joined":
				// Both if results meet at the merge, so the two resources
				// share one release point. Release them newest first: the
				// second if ran later, and its result has the larger local.
				if len(blocks) != 2 || blocks[0] != blocks[1] {
					t.Fatalf("%s: the joined resources are not released at one point: %v:\n%s", fname, blocks, p.Dump())
				}
				preds := 0
				for _, blk := range f.Blocks {
					if slices.Contains(blk.Term.Targets, blocks[0]) {
						preds++
					}
				}
				if preds < 2 {
					t.Errorf("%s: the joined resources are released before the arms merge (%d predecessors):\n%s", fname, preds, p.Dump())
				}
				var ids []midir.LocalID
				for _, in := range f.Blocks[blocks[0]].Insts {
					if in.Op == midir.OpRelease && len(in.Args) == 1 && p.R.HoldsResource(f.Locals[in.Args[0]].Type) {
						ids = append(ids, in.Args[0])
					}
				}
				if len(ids) != 2 || ids[0] < ids[1] {
					t.Errorf("%s: joined releases %v, want the later-created temporary first:\n%s", fname, ids, p.Dump())
				}
			case "main.looped":
				// Both loop resources are released in the iteration, not
				// after it, once per resource.
				succ := successors(f)
				cyclic := true
				for _, b := range blocks {
					cyclic = cyclic && reaches(succ, b, b)
				}
				if !cyclic {
					t.Errorf("%s: the loop resources are released outside the loop body: %v:\n%s", fname, blocks, p.Dump())
				}
			}
		})
	}
}
