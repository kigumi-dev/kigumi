package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
)

// sharedAliasSource builds `type A = P[P[...Int]]`: 2*depth type nodes, but 2^depth paths from A to Int.
func sharedAliasSource(depth int) string {
	return "type P[T] = (T, T)\ntype A = " + strings.Repeat("P[", depth) + "Int" + strings.Repeat("]", depth) +
		"\nfn f(x: A) -> Int { 0 }\nprint \"ok\"\n"
}

// TestSharedTypeDAGFiniteSizeWalkStaysLinear pins the finite-size check to one visit per type node. Walking every
// path of the shared DAG instead costs 2^depth visits: depth 24 takes minutes and allocates gigabytes.
func TestSharedTypeDAGFiniteSizeWalkStaysLinear(t *testing.T) {
	steps := map[int]int{}
	for _, depth := range []int{12, 24} {
		r := sem.CheckWith(buildModule(t, fixture{sections: []section{{name: "src", body: sharedAliasSource(depth)}}}), schema.GenerateDerive)
		if r.HasErrors() {
			t.Fatalf("the shared type DAG should check cleanly:\n%s", r.Render())
		}
		steps[depth] = r.FiniteSizeSteps()
		t.Logf("depth %d: %d containsByValue visits", depth, steps[depth])
	}
	// std contributes the same visits to both runs, so the growth is the generated module's alone: 12 more levels
	// add a constant number of nodes per level, never 2^depth paths.
	if grew := steps[24] - steps[12]; grew > 16*12 {
		t.Fatalf("12 more levels cost %d containsByValue visits, want at most %d", grew, 16*12)
	}
}

// nestedPairDiagnostic builds `let q: Int = pair(pair(...1))`: the E400 names the depth-nested tuple type, whose
// 2^depth paths all spell out if the shared DAG is expanded as a tree.
func nestedPairDiagnostic(t *testing.T, depth int) string {
	t.Helper()
	expr := "1"
	for range depth {
		expr = "pair(" + expr + ")"
	}
	src := "fn pair[T: Copy](x: T) -> (T, T) { (x, x) }\nlet p = " + expr + "\nlet q: Int = p\n"
	r := sem.CheckWith(buildModule(t, fixture{sections: []section{{name: "src", body: src}}}), schema.GenerateDerive)
	return r.Render()
}

// TestDeepTypeDiagnosticIsClipped pins one E400 naming a shared nested type to a bounded spelling. Expanding the
// DAG per path instead prints 2^depth paths: depth 20 writes 7 MB of stderr for a 227-byte source.
func TestDeepTypeDiagnosticIsClipped(t *testing.T) {
	const budget = 8 << 10
	shallow := nestedPairDiagnostic(t, 10)
	deep := nestedPairDiagnostic(t, 20)
	for depth, text := range map[int]string{10: shallow, 20: deep} {
		if n := strings.Count(text, "E400"); n != 1 {
			t.Fatalf("depth %d: got %d E400 diagnostics, want 1:\n%s", depth, n, text)
		}
		if len(text) >= budget {
			t.Fatalf("depth %d: the E400 message is %d bytes, want under %d", depth, len(text), budget)
		}
		if !strings.Contains(text, "…") {
			t.Fatalf("depth %d: a clipped type ends its elision with `…`:\n%s", depth, text)
		}
	}
	// The clip is depth-independent: ten more levels of sharing add no output.
	if shallow != deep {
		t.Fatalf("depth 20 renders %d bytes, depth 10 renders %d; the clip should not grow with depth", len(deep), len(shallow))
	}
	t.Logf("E400 is %d bytes at depth 10 and 20", len(deep))
}
