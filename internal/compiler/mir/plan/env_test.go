package plan_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
)

const envSrc = `interface Show { fn show(self) -> String }
type W[T] = { pub n Int }
fn W[T].show(self) -> String { "w" }
fn W[T].same(self, s: &Show) -> Bool { s is W(_) }
type Wrap[T] = { pub n Int }
fn Wrap[T].step(self, n: Int, acc: Int) -> Int { nest[Wrap[T]](n, acc) }
fn nest[T](n: Int, acc: Int) -> Int {
    let q: Show = W[T] { n: 0 }
    let a = if W[T] { n: 0 }.same(&q) { acc + 1 } else { acc }
    if n == 0 { return a }
    Wrap[T] { n: 0 }.step(n - 1, a)
}
fn sum[T](n: Int) -> Int { n + 1 }
fn later[T](n: Int) -> Int {
    let f = (k: Int) => {
        let q: Show = W[T] { n: k }
        W[T] { n: 0 }.same(&q)
    }
    if f(n) { 1 } else { 0 }
}
fn main() -> Void! {
    print "${nest[Int](10, 0)} ${sum[Int](1)} ${later[String](2)}"
}
`

func funcNamed(t *testing.T, p *mir.Program, name string) *midir.Func {
	t.Helper()
	for _, f := range p.Funcs {
		if _, minted := p.R.InstanceOf(f.Ent); !minted && p.R.Entity(f.Ent).Name == name && p.R.Entity(f.Ent).Kind == sem.EntFn {
			return f
		}
	}
	t.Fatalf("no body %s", name)
	return nil
}

func TestEnvGoesOnlyToBodiesThatComposeRuntimeTypes(t *testing.T) {
	p := specialized(t, envSrc)
	for name, want := range map[string]bool{"nest": true, "same": true, "step": true, "later": true, "sum": false} {
		if got := funcNamed(t, p, name).HasEnv; got != want {
			t.Errorf("%s takes an environment: %v, want %v", name, got, want)
		}
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestEnvRunsPolymorphicRecursionOnTheSharedBody(t *testing.T) {
	p := specialized(t, envSrc)
	if len(p.Diags) != 0 {
		t.Errorf("diagnostics = %+v, want none", p.Diags)
	}
	boxes := 0
	for _, blk := range funcNamed(t, p, "nest").Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBox && len(in.Rt) == 1 {
				boxes++
			}
		}
	}
	if boxes != 1 {
		t.Errorf("%d boxes in nest compose their runtime type, want 1", boxes)
	}
}

func TestEnvReachesALambdaAsACapture(t *testing.T) {
	p := specialized(t, envSrc)
	later := funcNamed(t, p, "later")
	var made *midir.Inst
	for _, blk := range later.Blocks {
		for i := range blk.Insts {
			if in := &blk.Insts[i]; in.Op == midir.OpClosure && p.R.Entity(in.Ent).Kind == sem.EntClosure {
				made = in
			}
		}
	}
	if made == nil {
		t.Fatalf("later builds no lambda:\n%s", p.Dump())
	}
	body := p.ByEnt[made.Ent]
	if body == nil || !body.HasEnv || made.Args[len(made.Args)-1] != later.Env {
		t.Fatalf("the lambda does not capture later's environment last: %v", made.Args)
	}
	if env := body.Locals[body.Env].Type; !p.R.Types.IsRtType(env) {
		t.Errorf("the lambda holds its environment as %s, not a runtime type", p.R.TypeString(env))
	}
	for _, f := range p.Funcs {
		if f.Ent == p.R.ClosureOrigin(made.Ent) && f.Ent != made.Ent {
			t.Errorf("the lambda's source body %s stayed beside its copy", f.Name)
		}
	}
}

func TestEnvKeepsRuntimeTypesUncounted(t *testing.T) {
	p := specialized(t, envSrc)
	for _, f := range p.Funcs {
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				counts := in.Op == midir.OpDrop || in.Op == midir.OpRelease || in.Op == midir.OpShare || in.Op == midir.OpCopy
				if counts && len(in.Args) > 0 && p.R.Types.IsRtType(f.Locals[in.Args[0]].Type) {
					t.Errorf("%s counts the runtime type %%%s: %s", f.Name, f.Locals[in.Args[0]].Name, in.Op)
				}
			}
		}
	}
}

func TestEnvSizesTheFrameOfAnAsyncBody(t *testing.T) {
	p := specialized(t, `import task from std/task
interface Show { fn show(self) -> String }
type W[T] = { pub n Int }
fn W[T].show(self) -> String { "w" }
fn W[T].same(self, s: &Show) -> Bool { s is W(_) }
async fn chk[T](p: Int) -> Bool {
    let want: Show = W[Int] { n: 0 }
    let f = (s: &Show) => W[T] { n: 0 }.same(s)
    f(&want)
}
async fn root() -> Bool { await chk[Int](1) }
let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(root())}"
`)
	chk := funcNamed(t, p, "chk")
	if !chk.HasEnv || len(chk.Slot) != len(chk.Locals) {
		t.Errorf("chk has %d frame slots for %d locals", len(chk.Slot), len(chk.Locals))
	}
}

func TestEnvStandsForItsOwnType(t *testing.T) {
	p := specialized(t, envSrc)
	for _, name := range []string{"same", "step"} {
		for _, blk := range funcNamed(t, p, name).Blocks {
			for _, in := range blk.Insts {
				if in.Op == midir.OpBuiltin && (in.Str == "type.of" || in.Str == "type.param") {
					t.Errorf("%s composes %s, its own environment", name, p.R.TypeString(in.Type))
				}
			}
		}
	}
}

func TestEnvRetypesValuesHoldingALambda(t *testing.T) {
	p := specialized(t, `interface Show { fn show(self) -> String }
type W[T] = { pub n Int }
fn W[T].show(self) -> String { "w" }
fn W[T].same(self, s: &Show) -> Bool { s is W(_) }
type V[F] = A(F) | B(Int)
fn held[T](s: &Show) -> Bool {
    let o = Some((t: &Show) => W[T] { n: 2 }.same(t))
    let v = V.A((t: &Show) => W[T] { n: 0 }.same(t))
    match o {
        Some(g) -> g(s)
        None -> v is V.B(_)
    }
}
let wi: Show = W[Int] { n: 0 }
print "${held[Int](&wi)}"
`)
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("verify: %v", err)
	}
}
