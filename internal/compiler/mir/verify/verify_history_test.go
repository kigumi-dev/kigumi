package verify_test

import (
	"slices"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

const weakDefsSource = `fn pick(o: Option[Int]) -> Int {
    if o is Some(x) && x > 1 { x } else { 0 }
}
fn unused() -> Int { 7 }
print "${pick(Some(2))}"
`

func TestUnchangedBodyKeepsItsVerifiedFlow(t *testing.T) {
	p := mirfixture.Planned(t, weakDefsSource)
	f := mirfixture.FuncNamed(t, p, "main.pick")
	var s mirverify.Session
	if err := s.Run(p); err != nil {
		t.Fatal(err)
	}
	hash, weak, ok := s.Flow(f)
	if !ok || !slices.Contains(weak, true) {
		t.Fatalf("pick must be remembered with a weakly defined binding, got %v %v", ok, weak)
	}
	// verifyDefs only ever produces a slice as long as f.Locals.
	marker := make([]bool, len(f.Locals)+1)
	s.SetFlow(f, hash, marker)
	if err := s.Run(p); err != nil {
		t.Fatal(err)
	}
	if _, got, _ := s.Flow(f); len(got) != len(marker) {
		t.Fatalf("an unchanged body must skip verifyDefs and hand its previous weakDefs on, got %v", got)
	}
	s.SetFlow(f, hash+1, marker)
	if err := s.Run(p); err != nil {
		t.Fatal(err)
	}
	if gotHash, got, _ := s.Flow(f); gotHash != hash || !slices.Equal(got, weak) {
		t.Fatalf("a changed body must be walked again, got hash %x weak %v, want %x %v", gotHash, got, hash, weak)
	}
}

func TestDroppedFuncLeavesTheHistory(t *testing.T) {
	p := mirfixture.Planned(t, weakDefsSource)
	var s mirverify.Session
	if err := s.Run(p); err != nil {
		t.Fatal(err)
	}
	unused := mirfixture.FuncNamed(t, p, "main.unused")
	if _, _, ok := s.Flow(unused); !ok {
		t.Fatal("unused was not remembered")
	}
	p.Funcs = slices.DeleteFunc(p.Funcs, func(f *midir.Func) bool { return f == unused })
	if err := s.Run(p); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := s.Flow(unused); ok {
		t.Fatal("a Func no longer in the program stayed in the history")
	}
	if got, want := s.Remembered(), len(p.AllFuncs()); got != want {
		t.Fatalf("history holds %d bodies, the program %d", got, want)
	}
}
