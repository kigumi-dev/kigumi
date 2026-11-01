package sem_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
)

func TestRootBridgeForwardBindingsIsolateCallSites(t *testing.T) {
	const prelude = `import task from std/task
async fn tick() -> Int { 1 }
fn drive() -> Int { task.currentThreadExecutor().runToCompletion(tick()) }
fn safe() -> Int { 1 }
fn leaf(f: escaping fn() -> Int) -> Int { f() }
fn split(f: escaping fn() -> Int, g: escaping fn() -> Int) -> Int { leaf(f) + leaf(g) }
`
	for _, tc := range []struct {
		first, second string
		blocked       bool
	}{{"safe", "safe", false}, {"safe", "drive", true}, {"drive", "safe", true}, {"drive", "drive", true}} {
		t.Run(tc.first+"_"+tc.second, func(t *testing.T) {
			body := prelude + fmt.Sprintf("async fn work() -> Int { split(%s, %s) }\n", tc.first, tc.second)
			fx := fixture{sections: []section{{name: "src", body: body}}}
			r := sem.CheckWith(buildModule(t, fx), schema.GenerateDerive)
			if got := strings.Count(r.Render(), "error[E1291]"); (got == 1) != tc.blocked || got > 1 || r.HasErrors() != tc.blocked {
				t.Fatalf("blocked = %v, diagnostics:\n%s", tc.blocked, r.Render())
			}
		})
	}
}

func duplicatedHelperChain(depth int) string {
	var src strings.Builder
	src.WriteString("type R = { pub x Int }\nfn h0(r: &mut R, f: escaping fn() -> Int) -> Int { r.x + f() }\n")
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&src, "fn h%d(r: &mut R, f: escaping fn() -> Int) -> Int { h%d(r, f) + h%d(r, f) }\n", i, i-1, i-1)
	}
	fmt.Fprintf(&src, "fn main() { let mut r = R { x: 1 }; print \"${h%d(&mut r, () => 2)}\" }\n", depth)
	return src.String()
}

func TestRootBridgeForwardFramesGrowLinearlyOnDuplicatedHelpers(t *testing.T) {
	frames := map[int]int{}
	for _, depth := range []int{10, 20} {
		r := sem.CheckWith(buildModule(t, fixture{sections: []section{{name: "src", body: duplicatedHelperChain(depth)}}}), schema.GenerateDerive)
		if r.HasErrors() {
			t.Fatal(r.Render())
		}
		frames[depth] = sem.RootForwardFrames(r)
		t.Logf("depth %d: %d calleeHopsWith frames", depth, frames[depth])
	}
	if frames[10] == 0 || frames[20] > 2*frames[10] {
		t.Fatalf("frames %v: doubling the chain may at most double the forwarding walk", frames)
	}
}
