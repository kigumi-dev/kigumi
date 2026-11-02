package verify_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const lifetimeOnlySrc = `
fn length['a](value: &'a String) -> usize { value.len() }
fn viaLet() -> usize {
    let fs = Array.of(length)
    let gs: Array[fn(&String) -> usize] = fs
    let s = "abc"
    gs[0](&s)
}
fn retLocal() -> Array[fn(&String) -> usize] {
    let fs = Array.of(length)
    fs
}
print "${viaLet()} ${retLocal().len()}"
`

// A container coercion that only erases reference lifetimes lowers to a plain copy or return, with no conversion step.
func TestVerifierTakesTypesDifferingOnlyInRefLifetimesAsOne(t *testing.T) {
	p := mirfixture.Built(t, lifetimeOnlySrc)
	f := mirfixture.FuncNamed(t, p, "main.viaLet")
	var from, to sem.TypeID
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if (in.Op == midir.OpCopy || in.Op == midir.OpMove) && f.Locals[in.Dst].Type != f.Locals[in.Args[0]].Type {
				from, to = f.Locals[in.Args[0]].Type, f.Locals[in.Dst].Type
			}
		}
	}
	if from == 0 {
		t.Fatalf("main.viaLet has no copy between types differing in reference lifetimes:\n%s", p.Dump())
	}
	if !p.SameType(from, to) {
		t.Errorf("SameType(%s, %s) = false", p.R.TypeString(from), p.R.TypeString(to))
	}
	tt := p.R.Types
	other := tt.Named(tt.ArrayEnt(), []sem.TypeID{tt.Fn([]sem.TypeID{sem.TyString}, sem.TyUsize, 0, false)})
	if p.SameType(from, other) {
		t.Errorf("SameType(%s, %s) = true", p.R.TypeString(from), p.R.TypeString(other))
	}
}
