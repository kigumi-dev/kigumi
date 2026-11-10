package plan_test

import (
	"kigumi/internal/devkit/fixture"
	"path/filepath"
	"sync/atomic"
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
	"kigumi/internal/devkit/semfixture"
)

func TestPlanRCCorpus(t *testing.T) {
	var files []string
	for _, glob := range []string{"../../../../testdata/mir/*.txtar", "../../../../testdata/run/*.txtar"} {
		fs, _ := filepath.Glob(glob)
		files = append(files, fs...)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	var sawRelease atomic.Bool
	t.Cleanup(func() {
		if !t.Failed() && !sawRelease.Load() {
			t.Error("plan-rc never inserted an OpRelease across the whole corpus")
		}
	})
	for _, path := range fixture.Shards(files) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			res, _, _ := mirfixture.CheckFixture(t, path, semfixture.LoadStd(t, "../../../../std"))
			prog := mirbuild.Program(res)
			if prog.HasErrors() {
				t.Skip("fixture expects ownership diagnostics")
			}
			if err := mirverify.Program(prog); err != nil {
				t.Fatalf("verify before plan-rc: %v", err)
			}
			if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
				t.Fatalf("plan-rc: %v\n%s", err, prog.Dump())
			}
			var funcs []*midir.Func
			funcs = append(funcs, prog.Funcs...)
			for _, cf := range prog.Comptime {
				funcs = append(funcs, cf.Func)
			}
			for _, f := range funcs {
				for _, blk := range f.Blocks {
					for _, in := range blk.Insts {
						if in.Op == midir.OpCopy {
							t.Errorf("%s: OpCopy survived plan-rc", f.Name)
						}
						if in.Op == midir.OpRelease {
							sawRelease.Store(true)
						}
					}
				}
			}
		})
	}
}

// rcLocals builds a locals slice where isNamed marks which indices carry a
// nonzero Ent (as if a real declared local), the rest being temporaries.
func rcLocals(t sem.TypeID, ent sem.EntityID, named ...bool) []midir.Local {
	ls := make([]midir.Local, len(named))
	for i, n := range named {
		ls[i] = midir.Local{Name: "t", Type: t}
		if n {
			ls[i].Ent = ent
		}
	}
	return ls
}

// planRC skips the Verify calls Apply makes: these fixtures isolate one decision and are not valid functions.
func planRC(t *testing.T, f *midir.Func, p *mir.Program) {
	t.Helper()
	p.Funcs = []*midir.Func{f}
	if err := mirplan.RC(p); err != nil {
		t.Fatalf("plan-rc: %v", err)
	}
}

// TestPlanRCAlias: copying a temporary (never a named local) is a pure
// alias, no runtime call.
func TestPlanRCAlias(t *testing.T) {
	p := mirfixture.Program(t)
	locals := rcLocals(sem.TyI64, 0, false, false)
	f := mirfixture.Fn("alias", sem.TyI64, locals,
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}},
			{Op: midir.OpCopy, Dst: 1, Args: []midir.LocalID{0}},
		}, Term: mirfixture.Ret(1)})
	planRC(t, f, p)
	if op := f.Blocks[0].Insts[1].Op; op != midir.OpAlias {
		t.Fatalf("got %s, want alias", op)
	}
}

// TestPlanRCShareFlag: an OpCopy built with Share (a match arm's own
// reference) always becomes an eager OpShare.
func TestPlanRCShareFlag(t *testing.T) {
	p := mirfixture.Program(t)
	locals := rcLocals(sem.TyI64, 0, false, false)
	f := mirfixture.Fn("shareflag", sem.TyI64, locals,
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}},
			{Op: midir.OpCopy, Dst: 1, Args: []midir.LocalID{0}, Share: true},
		}, Term: mirfixture.Ret(1)})
	planRC(t, f, p)
	in := f.Blocks[0].Insts[1]
	if in.Op != midir.OpShare || !in.Copy {
		t.Fatalf("got %s copy=%v, want share copy=true", in.Op, in.Copy)
	}
}

// TestPlanRCShareDefault: copying a named, move-only local into another
// named local shares by retaining (not duplicating) it.
func TestPlanRCShareDefault(t *testing.T) {
	p := mirfixture.Program(t)
	res := mirfixture.Resource(p)
	ent := p.R.Types.ErrorEnt()
	locals := rcLocals(res, ent, true, true)
	f := mirfixture.Fn("sharedefault", res, locals,
		midir.Block{Insts: []midir.Inst{{Op: midir.OpCopy, Dst: 1, Args: []midir.LocalID{0}}}, Term: midir.Term{Op: midir.TermReturn, Args: []midir.LocalID{0}}})
	f.Params = []midir.LocalID{0}
	planRC(t, f, p)
	in := f.Blocks[0].Insts[0]
	if in.Op != midir.OpShare || in.Copy {
		t.Fatalf("got %s copy=%v, want share copy=false (retain)", in.Op, in.Copy)
	}
}

// TestPlanRCBorrowNoShare: copying a value produced by a borrow is a pure
// alias even though the borrow's own destination is named.
func TestPlanRCBorrowNoShare(t *testing.T) {
	p := mirfixture.Program(t)
	ent := p.R.Types.ErrorEnt()
	ref := p.R.Types.Ref(sem.TyI64, false)
	locals := []midir.Local{{Name: "x", Type: sem.TyI64, Ent: ent}, {Name: "r", Type: ref, Ent: ent}, {Name: "r2", Type: ref, Ent: ent}}
	f := mirfixture.Fn("borrownoshare", sem.TyUnit, locals,
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpBorrow, Dst: 1, Args: []midir.LocalID{0}},
			{Op: midir.OpCopy, Dst: 2, Args: []midir.LocalID{1}},
		}, Term: mirfixture.Ret(2)})
	planRC(t, f, p)
	if op := f.Blocks[0].Insts[1].Op; op != midir.OpAlias {
		t.Fatalf("got %s, want alias (a reference is never shared)", op)
	}
}

func TestPlanRCArgShare(t *testing.T) {
	p := mirfixture.Program(t)
	ent := p.R.Types.ErrorEnt()
	locals := rcLocals(sem.TyI64, ent, true, false, false)
	f := mirfixture.Fn("argshare", sem.TyI64, locals,
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpCopy, Dst: 1, Args: []midir.LocalID{0}},
			{Op: midir.OpTag, Dst: 2, Args: []midir.LocalID{1}},
		}, Term: mirfixture.Ret(2)})
	f.Params = []midir.LocalID{0}
	planRC(t, f, p)
	insts := f.Blocks[0].Insts
	if len(insts) != 3 {
		t.Fatalf("got %d instructions, want 3 (alias, share, tag): %s", len(insts), p.Dump())
	}
	if insts[0].Op != midir.OpAlias {
		t.Fatalf("insts[0] = %s, want alias", insts[0].Op)
	}
	if insts[1].Op != midir.OpShare || !insts[1].Copy || insts[1].Args[0] != 1 {
		t.Fatalf("insts[1] = %+v, want an eager copy-share of local 1", insts[1])
	}
	if insts[2].Op != midir.OpTag || insts[2].Args[0] != insts[1].Dst {
		t.Fatalf("insts[2] = %+v, want tag of the shared value", insts[2])
	}
}

// TestPlanRCReturnShare: returning a named local the scope still owns
// takes its own reference; returning a temporary does not.
func TestPlanRCReturnShare(t *testing.T) {
	p := mirfixture.Program(t)
	ent := p.R.Types.ErrorEnt()
	locals := rcLocals(sem.TyI64, ent, true)
	f := mirfixture.Fn("returnshare", sem.TyI64, locals, midir.Block{Term: mirfixture.Ret(0)})
	f.Params = []midir.LocalID{0}
	planRC(t, f, p)
	insts := f.Blocks[0].Insts
	if len(insts) != 1 || insts[0].Op != midir.OpShare {
		t.Fatalf("got %v, want one share before the return", insts)
	}
	if got := f.Blocks[0].Term.Args[0]; got != insts[0].Dst {
		t.Fatalf("return argument %d, want the shared value %d", got, insts[0].Dst)
	}

	locals2 := rcLocals(sem.TyI64, 0, false)
	f2 := mirfixture.Fn("returntemp", sem.TyI64, locals2, midir.Block{Insts: []midir.Inst{
		{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}},
	}, Term: mirfixture.Ret(0)})
	planRC(t, f2, p)
	if insts := f2.Blocks[0].Insts; len(insts) != 1 {
		t.Fatalf("got %d instructions, want no share for a temporary return: %s", len(insts), p.Dump())
	}
}

// A chain of two OpAlias steps must still resolve to the named local, not just one.
func TestPlanRCAliasChainReturnShare(t *testing.T) {
	p := mirfixture.Program(t)
	ent := p.R.Types.ErrorEnt()
	ref := p.R.Types.Ref(sem.TyF64, false)
	locals := []midir.Local{{Name: "x", Type: ref, Ent: ent}, {Name: "t1", Type: sem.TyF64}, {Name: "t2", Type: sem.TyF64}}
	f := mirfixture.Fn("aliaschain", sem.TyF64, locals,
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpAlias, Dst: 1, Args: []midir.LocalID{0}},
			{Op: midir.OpAlias, Dst: 2, Args: []midir.LocalID{1}},
		}, Term: mirfixture.Ret(2)})
	f.Params = []midir.LocalID{0}
	planRC(t, f, p)
	insts := f.Blocks[0].Insts
	if len(insts) != 3 || insts[2].Op != midir.OpShare {
		t.Fatalf("got %v, want the two aliases plus one share before the return", insts)
	}
	if got := f.Blocks[0].Term.Args[0]; got != insts[2].Dst {
		t.Fatalf("return argument %d, want the shared value %d", got, insts[2].Dst)
	}
}

func TestPlanRCReleasesWhatASharedCopyAndAClosureLeave(t *testing.T) {
	prog := mirbuild.Program(mirfixture.CheckSourceMir(t, `
type Pt = {
    mut x Int
}

fn run() {
    let mut rec = Pt { x: 1 }
    let old = rec
    let mut bump = () => {
        rec.x = rec.x + 1
    }
    bump()
    print "${old.x} ${rec.x}"
}

fn main() -> Void! {}
`))
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatal(err)
	}
	run := mirfixture.FuncNamed(t, prog, "main.run")
	for _, blk := range run.Blocks {
		released := map[midir.LocalID]bool{}
		for _, in := range blk.Insts {
			if in.Op == midir.OpRelease {
				released[in.Args[0]] = true
			}
		}
		cellRead := map[midir.LocalID]bool{}
		shared := map[midir.LocalID]bool{}
		for _, in := range blk.Insts {
			switch in.Op {
			case midir.OpCellGet:
				cellRead[in.Dst] = true
			case midir.OpShare:
				shared[in.Dst] = true
				if cellRead[in.Args[0]] && !released[in.Args[0]] {
					t.Errorf("the read %%t%d a share copies is never released:\n%s", in.Args[0], prog.Dump())
				}
			case midir.OpClosure:
				for _, a := range in.Args {
					if shared[a] && !released[a] {
						t.Errorf("the share %%t%d a closure retains again is never released:\n%s", a, prog.Dump())
					}
				}
			}
		}
	}
}
