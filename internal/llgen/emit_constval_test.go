package llgen_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/mir"
)

// The implicit `Ok(unit)` of a `-> Unit!` body and `Err(AllocError.OutOfMemory)`
// are the same value everywhere they are built, so neither needs storage.
func TestFixedPayloadVariantBuildsWithoutAllocating(t *testing.T) {
	ir := emitIR(t, `
fn unitOk(bad: Bool) -> Result[Unit, AllocError] {
    if bad {
        return Err(AllocError.OutOfMemory)
    }
}

fn main() -> Unit! {}
`)
	body := bodyOf(t, ir, `define ptr @"main.unitOk"(`)
	checkNoHeap(t, "main.unitOk", body)
	if n := strings.Count(body, `@".constval.`); n < 2 {
		t.Errorf("want both the Ok and the Err from a shared slot, found %d:\n%s", n, body)
	}
	init := bodyOf(t, ir, "define void @kg_init_immortals() {")
	if !strings.Contains(init, "@rt_const_variant(") {
		t.Errorf("kg_init_immortals does not fill the shared variants:\n%s", init)
	}
	if !strings.Contains(ir, `.payload" = internal global [1 x ptr] zeroinitializer`) {
		t.Error("no payload storage reserved beside a shared variant")
	}
}

// A fallible-Unit body ending in a call returns the same one Ok(unit):
// the call's own Unit value is released at the site, not stored.
func TestUnitTailOkBuildsWithoutAllocating(t *testing.T) {
	ir := emitIR(t, `
fn side(n: Int) -> Unit {}

fn wrap(n: Int) -> Result[Unit, AllocError] {
    side(n)
}

fn main() -> Unit! {}
`)
	body := bodyOf(t, ir, `define ptr @"main.wrap"(`)
	checkNoHeap(t, "main.wrap", body)
	if !strings.Contains(body, `@".constval.`) {
		t.Errorf("the Ok wrapping a called Unit must come from the shared slot:\n%s", body)
	}
}

func TestFixedPayloadSharedWithoutOptionTake(t *testing.T) {
	ir := emitIR(t, `
type Mark = Cold | Hot

fn here() -> Option[Mark] { Some(Cold) }

fn main() -> Unit! {}
`)
	body := bodyOf(t, ir, `define ptr @"main.here"(`)
	checkNoHeap(t, "main.here", body)
}

// take() empties the receiver through its write-back cell, not through the
// Some it found, so a program that calls it still shares one Some.
func TestOptionTakeSharesSome(t *testing.T) {
	ir := emitIR(t, `
type Mark = Cold | Hot

fn here() -> Option[Mark] { Some(Cold) }

fn main() -> Unit! {
    let mut o = here()
    _ = o.take()
}
`)
	body := bodyOf(t, ir, `define ptr @"main.here"(`)
	checkNoHeap(t, "main.here", body)
	if !strings.Contains(body, `@".constval.`) {
		t.Errorf("a Some whose payload is fixed must come from the shared slot:\n%s", body)
	}
}

var varRewrite = regexp.MustCompile(`(?m)^\s*(a[0-9]->u\.var\.[a-z]+ = [^;]*;)`)

// Sharing one value per construction site is only sound while no runtime
// op rewrites a variant in place.
func TestVariantRewriteInventory(t *testing.T) {
	src, err := os.ReadFile("runtime/rt_core.c")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range varRewrite.FindAllStringSubmatch(string(src), -1) {
		got = append(got, m[1])
	}
	if len(got) > 0 {
		t.Errorf("rt_core.c rewrites variants in place at:\n%s", strings.Join(got, "\n"))
	}
	if _, ok := mir.StdOpIdOf("OPTION_TAKE"); !ok {
		t.Error("the registry no longer names OPTION_TAKE")
	}
}

// A closure call used to build a heap array for its arguments, which made
// calling one in an ISR handler or a `noalloc` body allocate unseen.
func TestInvokeBuildsNoArgumentArray(t *testing.T) {
	src, err := os.ReadFile("runtime/rt_core.c")
	if err != nil {
		t.Fatal(err)
	}
	body := cBody(t, string(src), "val invoke(adapter_fn fn, val env, int n, val* args) {")
	if strings.Contains(body, "rt_array(") {
		t.Errorf("invoke still allocates the argument array:\n%s", body)
	}
}

// Reporting a failed allocation must not need one, and must not race a
// handler that preempted a first-use initializer.
func TestAllocErrorBuiltInRtInit(t *testing.T) {
	src, err := os.ReadFile("runtime/rt_core.c")
	if err != nil {
		t.Fatal(err)
	}
	if body := cBody(t, string(src), "static val alloc_error(void) {"); strings.Contains(body, "ready") {
		t.Errorf("alloc_error still builds its value on first use:\n%s", body)
	}
	if body := cBody(t, string(src), "void rt_init(int argc, char** argv, int ntypes, rt_type** types) {"); !strings.Contains(body, "g_alloc_error") {
		t.Errorf("rt_init does not build the allocation error:\n%s", body)
	}
}

// An artifact with no entry is started by an embedder calling kigumi_main
// once; without rt_init there, the variant table stays empty.
func TestEntrylessArtifactInitializesRuntime(t *testing.T) {
	ir := emitIR(t, `
export(C) {
    pub fn poke(n: Int) -> Int { n + 1 }
}
`)
	body := bodyOf(t, ir, "define i32 @main(")
	for _, want := range []string{"@kg_init_immortals()", "@rt_init(", "@kg_init_statics()"} {
		if !strings.Contains(body, want) {
			t.Errorf("an entry-less artifact must still call %s:\n%s", want, body)
		}
	}
}
