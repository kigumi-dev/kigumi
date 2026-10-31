package llgen_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A literal's value is fixed before the program starts, so building one
// must not reach the allocator, in an ISR handler or anywhere else.
func TestLiteralsBuildWithoutAllocating(t *testing.T) {
	ir := emitIR(t, `
fn takes(s: String, n: Int, c: Char, f: Float) -> usize { s.len() }

fn build() -> usize {
    takes("hello", 42, 'z', 2.5)
}

fn main() -> Void! {}
`)
	body := fnBody(t, ir, "main.build")
	checkNoHeap(t, "main.build", body)
	for _, bad := range []string{"@rt_str(", "@rt_int(", "@rt_char(", "@rt_float("} {
		if strings.Contains(body, bad) {
			t.Errorf("main.build calls %s for a literal:\n%s", bad, body)
		}
	}
	if n := strings.Count(body, `@".constval.`); n < 1 {
		t.Errorf("want the String literal from a shared slot, found %d:\n%s", n, body)
	}
	if !strings.Contains(body, `(ptr %v1, i64 %v2, i32 %v3, double %v4)`) {
		t.Errorf("the scalar literals do not reach takes in registers:\n%s", body)
	}
	init := bodyOf(t, ir, "define void @kg_init_immortals() {")
	for _, want := range []string{"@rt_const_num(", "@rt_const_text("} {
		if !strings.Contains(init, want) {
			t.Errorf("kg_init_immortals does not call %s:\n%s", want, init)
		}
	}
}

// A literal reaching a parameter the callee keeps boxed is the immortal, not a fresh box.
func TestScalarLiteralArgumentUsesImmortal(t *testing.T) {
	ir := emitIR(t, `
fn step(n: Int) -> Int {
    let xs = Array.of(n, 2)
    (xs.get(0) || 0) + 1
}

fn run() -> Int {
    let k = 4
    step(k) + step(k)
}

fn main() -> Void! {}
`)
	body := fnBody(t, ir, "main.run")
	if n := strings.Count(body, "@rt_int("); n != 0 {
		t.Errorf("want nothing boxed, the sum leaves in a register; found %d rt_int calls:\n%s", n, body)
	}
	if n := strings.Count(body, `@".constval.`); n != 2 {
		t.Errorf("want both literal arguments from the shared slot, found %d:\n%s", n, body)
	}
}

// Forwarding the Result a `?` took apart costs nothing, and the Result holding a Fault is built in the frame.
func TestTryForwardsErrWithoutRebuilding(t *testing.T) {
	ir := emitIR(t, `
type Fault = Low | Named(why: String)

fn source(bad: Bool) -> Result[Int, Fault] {
    if bad { Err(Low) } else { Ok(1) }
}

fn forwards(bad: Bool) -> Result[Int, Fault] {
    let v = source(bad)?
    Ok(v + 1)
}

fn main() -> Void! {}
`)
	body := bodyOf(t, ir, `define %"kg.adt.Result[Int,main.Fault]" @"main.forwards"(`)
	checkNoHeap(t, "main.forwards", body)
}

var scalarRewrite = regexp.MustCompile(`(?m)^\s*((\S+)->u\.(?:i|f|c|b|s\.[a-z]+) = [^;]*;)`)

// Sharing one immortal per literal is only sound while no runtime op rewrites a scalar or
// String payload through the object it was built in; the Bytes rewrite is why b".." is not shared.
func TestScalarRewriteInventory(t *testing.T) {
	src, err := os.ReadFile("runtime/rt_core.c")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range scalarRewrite.FindAllStringSubmatch(string(src), -1) {
		switch {
		case m[2] == "v", m[2] == "c":
		case m[1] == "a0->u.s.data = d;":
		default:
			t.Errorf("rt_core.c rewrites a payload outside a constructor: %s", m[1])
		}
	}
}

// A generic call passes one witness per requirement; building those as
// closures allocated twice per call before any work was done.
func TestCaptureFreeWitnessIsStatic(t *testing.T) {
	ir := emitIR(t, `
fn largest[T: Ord](xs: &Array[T]) -> usize {
    let mut best: usize = 0
    for i in 0..xs.len() {
        if &xs[i] > &xs[best] { best = i }
    }
    best
}

fn main() -> Void! {
    let xs = Array.of[Int](3, 9, 4)
    print "${largest(&xs)}"
}
`)
	body := bodyOf(t, ir, `define %"kg.adt.Result[Void,std/prelude.Error]" @"main.main"(`)
	if strings.Contains(body, "@rt_std_bind(") {
		t.Errorf("a capture-free witness is still built per call:\n%s", body)
	}
	if !strings.Contains(body, `@".stdbind.`) {
		t.Errorf("the witness does not come from a reserved slot:\n%s", body)
	}
	init := bodyOf(t, ir, "define void @kg_init_immortals() {")
	if !strings.Contains(init, "@rt_stditem_init(") {
		t.Errorf("kg_init_immortals does not fill the witness slots:\n%s", init)
	}
}
