package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func TestParseStdOpRegistryRejects(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"id gap", "op 0 A primitive () -> unit\nop 2 B primitive () -> unit\n", "consecutive"},
		{"id out of order", "op 1 A primitive () -> unit\n", "consecutive"},
		{"duplicate name", "op 0 A primitive () -> unit\nop 1 A primitive () -> unit\n", "twice"},
		{"unknown kind", "op 0 A hosted () -> unit\n", "neither primitive nor provider"},
		{"unknown class", "op 0 A primitive (widget) -> unit\n", "unknown class"},
		{"missing result", "op 0 A primitive (int)\n", "->"},
		{"unknown capability", "op 0 A primitive () -> unit\ncap host.warp\n", "vocabulary"},
		{"key claimed twice", "op 0 A primitive () -> unit\nkey x.y\nop 1 B primitive () -> unit\nkey x.y\n", "claimed by both"},
		{"key without op", "key x.y\n", "needs an op"},
		{"unknown directive", "frob 1\n", "unknown directive"},
		{"unknown direct", "op 0 A primitive () -> unit\ndirect Bogus\n", "not a direct primitive"},
		{"bad callback", "op 0 A primitive () -> unit\ncallback x\n", "argument index"},
		{"scalar-box arity", "op 0 A primitive (named) -> any\nscalar-box 0\n", "type-argument index"},
		{"scalar-box negative", "op 0 A primitive (named) -> any\nscalar-box 0 -1\n", "non-negative"},
		{"scalar-box noninteger", "op 0 A primitive (named) -> any\nscalar-box x 0\n", "non-negative"},
		{"scalar-box argument", "op 0 A primitive (named) -> any\nscalar-box 1 0\n", "named argument"},
		{"scalar-box class", "op 0 A primitive (int) -> int\nscalar-box 0 0\n", "named argument"},
		{"scalar-box duplicate", "op 0 A primitive (named) -> any\nscalar-box 0 0\nscalar-box 0 0\n", "twice"},
		{"scalar-box retired", "op 0 A retired\nscalar-box 0 0\n", "retired"},
		{"retired with attributes", "op 0 A retired\nkey x.y\n", "retired"},
		{"retired with descriptor", "op 0 A retired () -> unit\n", "retired"},
		{"builtin flag", "builtin print (string) -> unit nope\n", "unknown flag"},
		{"builtin min", "builtin print (string) -> unit min=2\n", "exceeds"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := mir.ParseStdOpRegistry(c.text)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error mentioning %q", err, c.want)
			}
		})
	}
}

func TestStdOpScalarBoxSource(t *testing.T) {
	text := "op 0 A primitive (named) -> any\nscalar-box 0 0\n"
	reg, err := mir.ParseStdOpRegistry(text)
	if err != nil {
		t.Fatal(err)
	}
	if src := reg.Ops[0].ScalarBox; src == nil || src.Argument != 0 || src.TypeArg != 0 {
		t.Fatalf("scalar box source = %+v", src)
	}
	if !strings.Contains(reg.Canonical(), "scalar-box 0 0\n") {
		t.Fatal("canonical registry lost the scalar-box contract")
	}
	other, err := mir.ParseStdOpRegistry(strings.Replace(text, "scalar-box 0 0", "scalar-box 0 1", 1))
	if err != nil || reg.Canonical() == other.Canonical() {
		t.Fatal("changing the scalar-box type argument did not change the contract")
	}
	for _, op := range mir.Registry.Ops {
		if strings.HasPrefix(op.Name, "UCELL_ATOMIC_") && (op.ScalarBox == nil || op.ScalarBox.Argument != 0 || op.ScalarBox.TypeArg != 0) {
			t.Errorf("%s has no content-box contract", op.Name)
		}
	}
}

func TestStdOpRegistryRoundTrip(t *testing.T) {
	reg, err := mir.ParseStdOpRegistry("op 0 A primitive (int, string|bytes) -> option\ncap host.files\nkey a.b\nsuffix c\nprefix d.\ndirect Len\nelem\ncallback 2\nbuiltin x (any, int) -> int min=1 iter vm native=inline\n")
	if err != nil {
		t.Fatal(err)
	}
	op := reg.Ops[0]
	if op.Name != "A" || op.Cap != mir.CapHostFiles || len(op.Keys) != 1 || len(op.Suffixes) != 1 || len(op.Prefixes) != 1 {
		t.Fatalf("parsed op = %+v", op)
	}
	if got := op.Desc.Args[1].String(); got != "string|bytes" {
		t.Errorf("class set prints as %q", got)
	}
	if op.Direct != "Len" || !op.Elem || op.Callback != 2 {
		t.Fatalf("parsed attributes = %+v", op)
	}
	b := reg.Builtins[0]
	if b.Min != 1 || !b.Iter || !b.VM || b.Native != mir.NativeInline || len(b.Args) != 2 {
		t.Fatalf("parsed builtin = %+v", b)
	}
}

func TestRetiredOpKeepsItsIdAndResolvesNothing(t *testing.T) {
	reg, err := mir.ParseStdOpRegistry("op 0 A retired\nop 1 B primitive () -> unit\nkey b\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reg.Ops[0].Retired || reg.Ops[1].Retired || reg.Ops[1].ID != 1 || len(reg.Ops[0].Keys) != 0 {
		t.Fatalf("parsed ops = %+v", reg.Ops)
	}
	if got := reg.Canonical(); !strings.Contains(got, "op 0 A retired\n") {
		t.Errorf("canonical form drops the retired row: %q", got)
	}
}

func TestRegistryAttributesMatchTheirUses(t *testing.T) {
	want := map[string]bool{"ARRAY_GET": true, "ARRAY_FIRST": true, "ARRAY_LAST": true}
	for _, op := range mir.Registry.Ops {
		if op.Elem != want[op.Name] {
			t.Errorf("op %s: elem = %v", op.Name, op.Elem)
		}
		if got := op.Callback; got != wantCallback(op) {
			t.Errorf("op %s: callback = %d, want %d", op.Name, got, wantCallback(op))
		}
	}
}

// wantCallback is the index of the op's fn argument; the blocking pool's work runs on its own worker, so it has none.
func wantCallback(op mir.StdOpEntry) int {
	if op.Name == "BLOCKING_POOL_RUN" || op.Retired {
		return -1
	}
	for i, a := range op.Desc.Args {
		if a == 1<<mir.ClassFn {
			return i
		}
	}
	return -1
}

func TestRegistryIdsFollowNames(t *testing.T) {
	for i, op := range mir.Registry.Ops {
		if op.ID != i || mir.StdOpNames[i] != op.Name {
			t.Errorf("op %s: id %d at position %d", op.Name, op.ID, i)
		}
		id, ok := mir.StdOpIdOf(op.Name)
		if !ok || int(id) != i {
			t.Errorf("StdOpIdOf(%s) = %d, %v; want %d", op.Name, id, ok, i)
		}
	}
}

func TestOpDescriptorChecksOperandsAndResult(t *testing.T) {
	p := program(t)
	id, _ := mir.StdOpIdOf("ARRAY_LEN")
	locals := []mir.Local{{Name: "s", Type: sem.TyString}, {Name: "f", Type: sem.TyF64}}
	f := fn("desc", sem.TyUnit, locals)
	call := mir.Inst{Op: mir.OpCall, Dst: 1, Args: []mir.LocalID{0}, Prim: mir.PrimitiveId(id) + 1}
	if msg := p.OpDescMismatch(f, call); !strings.Contains(msg, "argument 0 is a string") {
		t.Errorf("a String receiver for ARRAY_LEN: %q", msg)
	}
	call.Args = nil
	if msg := p.OpDescMismatch(f, call); !strings.Contains(msg, "takes 1 arguments, got 0") {
		t.Errorf("a missing receiver for ARRAY_LEN: %q", msg)
	}
	f.Locals[0].Type = sem.TyUsize
	f.Locals[1].Type = sem.TyBool
	sqrt, _ := mir.StdOpIdOf("MATH_SQRT")
	call = mir.Inst{Op: mir.OpCall, Dst: 1, Args: []mir.LocalID{0}, Prim: mir.PrimitiveId(sqrt) + 1}
	if msg := p.OpDescMismatch(f, call); !strings.Contains(msg, "argument 0 is a int") {
		t.Errorf("an integer for MATH_SQRT: %q", msg)
	}
	f.Locals[0].Type = sem.TyF64
	if msg := p.OpDescMismatch(f, call); !strings.Contains(msg, "result is a bool") {
		t.Errorf("a Bool result for MATH_SQRT: %q", msg)
	}
	f.Locals[1].Type = sem.TyF64
	if msg := p.OpDescMismatch(f, call); msg != "" {
		t.Errorf("a well-typed MATH_SQRT call: %q", msg)
	}
}
