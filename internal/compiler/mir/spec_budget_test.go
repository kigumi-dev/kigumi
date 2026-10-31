package mir_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

func fanOut(levels int) string {
	var src strings.Builder
	src.WriteString("type P = {\n    pub x Int\n    pub y Int\n}\n\n")
	for i := 0; i < levels-1; i++ {
		fmt.Fprintf(&src, "fn g%d[T: Copy](x: T) -> Int { g%d((x, 1)) + g%d((2, x)) }\n", i, i+1, i+1)
	}
	fmt.Fprintf(&src, "fn g%d[T: Copy](x: T) -> Int { 1 }\n\n", levels-1)
	src.WriteString("fn run(n: Int) -> Int { g0(P { x: n, y: n }) }\n\nfn main() -> Void! {}\n")
	return src.String()
}

func mintedFor(r *sem.Result, prefix string) int {
	n := 0
	for id := range r.Entities {
		if in, ok := r.InstanceOf(sem.EntityID(id)); ok && strings.HasPrefix(r.Entity(in.Gen).Name, prefix) {
			n++
		}
	}
	return n
}

func TestSpecializeStopsAFanOutAtItsBudget(t *testing.T) {
	p := specialized(t, fanOut(12))
	if n := mintedFor(p.R, "g"); n > 257 {
		t.Errorf("minted %d instantiations, want at most 257", n)
	}
	if len(p.Diags) != 1 || p.Diags[0].Diag.Code != "W1711" || p.Diags[0].Diag.Severity != diag.Warning {
		t.Fatalf("diagnostics = %+v, want one W1711 warning", p.Diags)
	}
	if msg := p.Diags[0].Diag.Msg; !strings.Contains(msg, "calling `g0` with `main.P`") || !strings.Contains(msg, "more than 256 instantiations") {
		t.Errorf("message %q", msg)
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestSpecializeKeepsAFanOutWithinItsBudget(t *testing.T) {
	p := specialized(t, fanOut(6))
	if len(p.Diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none", p.Diags)
	}
	if n := len(instances(p)); n != 63 {
		t.Errorf("%d instantiations kept, want all 63:\n%s", n, strings.Join(instances(p), "\n"))
	}
}
