package llgen_test

import (
	"os"
	"strings"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

func emitIR(t *testing.T, src string) string {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatal(err)
	}
	return llgen.Emit(prog)
}

func bodyOf(t *testing.T, ir, header string) string {
	t.Helper()
	start := strings.Index(ir, header)
	if start < 0 {
		t.Fatalf("no %s in IR:\n%s", header, ir)
	}
	end := strings.Index(ir[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("%s is not closed:\n%s", header, ir)
	}
	return ir[start : start+end]
}

func checkNoHeap(t *testing.T, what, body string) {
	t.Helper()
	for _, bad := range []string{"@calloc", "@malloc", "@rt_record(", "@rt_variant(", "@rt_array(", "@rt_closure(", "@rt_closure_async(", "@rt_cell(", "@rt_box("} {
		if strings.Contains(body, bad) {
			t.Errorf("%s calls %s, which allocates:\n%s", what, bad, body)
		}
	}
}

func TestNullaryVariantBuildsWithoutAllocating(t *testing.T) {
	ir := emitIR(t, `
type Color = Red | Green | Blue

fn pick(n: Int) -> Color {
    if n == 0 { Red } else { Blue }
}

fn nothing() -> Int? {
    None
}

fn main() -> Unit! {}
`)
	for _, fn := range []string{`define ptr @"main.pick"(`, `define ptr @"main.nothing"(`} {
		body := bodyOf(t, ir, fn)
		checkNoHeap(t, fn, body)
		if !strings.Contains(body, "@rt_nullary(") {
			t.Errorf("%s does not build its variant through rt_nullary:\n%s", fn, body)
		}
	}
}

func TestCaptureFreeFunctionValueBuildsWithoutAllocating(t *testing.T) {
	ir := emitIR(t, `
fn twice(x: Int) -> Int { x + x }

fn apply(f: fn(Int) -> Int) -> Int { f(1) }

fn named() -> Int { apply(twice) }

fn lambda() -> Int { apply((x: Int) => x + 1) }

fn main() -> Unit! {}
`)
	for _, fn := range []string{`define ptr @"main.named"(`, `define ptr @"main.lambda"(`} {
		body := bodyOf(t, ir, fn)
		checkNoHeap(t, fn, body)
		if !strings.Contains(body, `$adapter.fnitem"`) {
			t.Errorf("%s does not take its function value from a reserved immortal slot:\n%s", fn, body)
		}
	}
	if !strings.Contains(ir, `@"main.twice$adapter.fnitem" = internal global [64 x i8] zeroinitializer`) {
		t.Errorf("no reserved slot for the `twice` function value:\n%s", ir)
	}
}

// An interrupt handler may construct any immortal, so every one of them is
// filled before the entry runs rather than at its first construction.
func TestImmortalsBuiltBeforeEntry(t *testing.T) {
	ir := emitIR(t, `
fn twice(x: Int) -> Int { x + x }

fn apply(f: fn(Int) -> Int) -> Int { f(1) }

fn main() -> Unit! {
    print "${apply(twice)}"
}
`)
	init := bodyOf(t, ir, "define void @kg_init_immortals() {")
	for _, want := range []string{
		"call void @rt_init_nullary(i32 ",
		`call void @rt_fnitem_init(ptr @"main.twice$adapter.fnitem", ptr @"main.twice$adapter", i32 0)`,
	} {
		if !strings.Contains(init, want) {
			t.Errorf("kg_init_immortals does not emit %q:\n%s", want, init)
		}
	}
	body := bodyOf(t, ir, "define i32 @main(")
	immortals, init0, statics := strings.Index(body, "@kg_init_immortals()"), strings.Index(body, "@rt_init("), strings.Index(body, "@kg_init_statics()")
	if immortals < 0 || immortals > init0 || init0 > statics {
		t.Errorf("main must fill the immortals before anything can read them:\n%s", body)
	}
}

// The runtime must not build an immortal at its first construction: a
// handler can preempt the constructor between the sentinel and the payload.
func TestRuntimeBuildsNoImmortalOnFirstUse(t *testing.T) {
	src, err := os.ReadFile("runtime/rt_core.c")
	if err != nil {
		t.Fatal(err)
	}
	for _, header := range []string{"val rt_nullary(rt_vdesc* d) {", "void rt_fnitem_init(void* slot, adapter_fn fn, int async) {"} {
		if body := cBody(t, string(src), header); strings.Contains(body, "rc ==") {
			t.Errorf("%s tests the sentinel it writes:\n%s", header, body)
		}
	}
	if !strings.Contains(string(src), "void rt_init_nullary(int ntypes, rt_type** types) {") {
		t.Error("rt_core.c no longer fills the payload-free variants up front")
	}
}

func cBody(t *testing.T, src, header string) string {
	t.Helper()
	start := strings.Index(src, header)
	if start < 0 {
		t.Fatalf("rt_core.c has no %s", header)
	}
	end := strings.Index(src[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("%s is not closed", header)
	}
	return src[start : start+end]
}

// A capture carries state, so its closure keeps the ordinary boxed form.
func TestCapturingClosureStillAllocates(t *testing.T) {
	ir := emitIR(t, `
fn apply(f: fn(Int) -> Int) -> Int { f(1) }

fn capturing(n: Int) -> Int { apply((x: Int) => x + n) }

fn main() -> Unit! {}
`)
	body := bodyOf(t, ir, `define ptr @"main.capturing"(`)
	if !strings.Contains(body, "@rt_closure(") {
		t.Errorf("a capturing closure must still go through rt_closure:\n%s", body)
	}
}

// `self = Dot` writes the caller's storage through the receiver cell, not
// the Dot it was handed, so that type's variants still share one immortal.
func TestWholeSelfAssignKeepsVariantsShared(t *testing.T) {
	ir := emitIR(t, `
type Shape = Circle(r: Float) | Dot

fn Shape.clear(mut self) -> Unit { self = Dot }

fn blank() -> Shape { Dot }

fn main() -> Unit! {}
`)
	body := bodyOf(t, ir, `define ptr @"main.blank"(`)
	checkNoHeap(t, "main.blank", body)
	if !strings.Contains(body, "@rt_nullary(") {
		t.Errorf("a payload-free variant must share the immortal:\n%s", body)
	}
	clear := bodyOf(t, ir, `define ptr @"main.Shape.clear"(`)
	if !strings.Contains(clear, "@rt_cellset(") {
		t.Errorf("`self = Dot` must store through the receiver cell:\n%s", clear)
	}
}
