package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

const directTickSrc = `
type Tick = Up(n: Int) | Down(n: Int) | Flat

fn value(t: Tick) -> Int {
    match t {
        Up(n) -> n
        Down(n) -> 0 - n
        Flat -> 0
    }
}

fn main() -> Void! { print "${value(Up(2))}" }
`

func TestFieldWriteOnADirectSumTypeIsRefused(t *testing.T) {
	prog := plannedProgram(t, directTickSrc)
	var f *midir.Func
	for _, c := range prog.Funcs {
		if c.Name == "main.value" {
			f = c
		}
	}
	if f == nil || !f.IsDirect(f.Params[0]) {
		t.Fatal("value should take its Tick unboxed")
	}
	f.Locals = append(f.Locals, midir.Local{Name: "n", Type: sem.TyI64})
	f.Direct = append(f.Direct, false)
	if len(f.Slot) > 0 {
		f.Slot = append(f.Slot, -1)
	}
	n := midir.LocalID(len(f.Locals) - 1)
	f.Blocks[0].Insts = append(f.Blocks[0].Insts,
		midir.Inst{Op: midir.OpConst, Dst: n, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}},
		midir.Inst{Op: midir.OpSetField, Args: []midir.LocalID{f.Params[0], n}, Index: 0})
	defer func() {
		msg, _ := recover().(string)
		if !strings.Contains(msg, "setfield names field 0 of") {
			t.Fatalf("emitting a field write on a direct sum type did not refuse it: %v", msg)
		}
	}()
	llgen.Emit(prog)
}
