package driver_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestSpecializationWarnsOnRunawayGrowth(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, `type Box[T] = {
    pub v T
}

fn Box[T: Copy].nest(self, n: Int) -> Int {
    if n == 0 { 0 } else { 1 + Box { v: Box { v: self.v } }.nest(n - 1) }
}

fn main() -> Void! {
    print "${Box { v: (1, 2) }.nest(3)}"
}
`)
	if strings.Count(out, "warning[W1710]") != 1 || !strings.Contains(out, "`nest` keeps being instantiated") || strings.Contains(out, "error") {
		t.Fatalf("expected one W1710 warning at the growing call and no error, got:\n%s", out)
	}
}

func TestSpecializationLeavesScalarRecursionShared(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, `type Box[T] = {
    pub v T
}

fn Box[T: Copy].nest(self, n: Int) -> Int {
    if n == 0 { 0 } else { 1 + Box { v: Box { v: self.v } }.nest(n - 1) }
}

fn main() -> Void! {
    print "${Box { v: 1 }.nest(3)}"
}
`)
	if out != "" {
		t.Fatalf("expected the scalar instantiation to keep the shared body quietly, got:\n%s", out)
	}
}

func TestSpecializationAcceptsALongChain(t *testing.T) {
	t.Parallel()
	var src strings.Builder
	src.WriteString("type Point = {\n    pub x Int\n    pub y Int\n}\n\n")
	for i := 1; i < 70; i++ {
		fmt.Fprintf(&src, "fn step%d[T](x: T) -> T { step%d(x) }\n", i, i+1)
	}
	src.WriteString("fn step70[T](x: T) -> T { x }\n\nfn main() -> Void! {\n    print \"${step1(Point { x: 1, y: 2 }).y}\"\n}\n")
	if out := checkAllocClaim(t, src.String()); strings.Contains(out, "error") {
		t.Fatalf("expected the chain to build, got:\n%s", out)
	}
}

func TestSpecializationWarnsWhenOneCallBringsInTooMany(t *testing.T) {
	t.Parallel()
	var src strings.Builder
	src.WriteString("type P = {\n    pub x Int\n    pub y Int\n}\n\n")
	for i := 0; i < 11; i++ {
		fmt.Fprintf(&src, "fn g%d[T: Copy](x: T) -> Int { g%d((x, 1)) + g%d((2, x)) }\n", i, i+1, i+1)
	}
	src.WriteString("fn g11[T: Copy](x: T) -> Int { 1 }\n\nfn main() -> Void! {\n    print \"${g0(P { x: 1, y: 2 })}\"\n}\n")
	out := checkAllocClaim(t, src.String())
	if strings.Count(out, "warning[W1711]") != 1 || !strings.Contains(out, "calling `g0` with `allocclaim.P` would bring in more than 256 instantiations") || strings.Contains(out, "error") {
		t.Fatalf("expected one W1711 warning at the call and no error, got:\n%s", out)
	}
}
