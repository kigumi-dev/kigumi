package mir_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func discardedFanOut(wasters int) string {
	var src strings.Builder
	src.WriteString("type P = {\n    pub x Int\n    pub y Int\n}\n\n")
	for i := 0; i < wasters; i++ {
		fmt.Fprintf(&src, "fn waste%d[T: Copy]() -> Int { 1 }\n", i)
	}
	src.WriteString("\nfn useful[T: Copy](x: T) -> T { x }\n\nfn top[T: Copy](x: T) -> Int {\n    let mut s = 0\n")
	for i := 0; i < wasters; i++ {
		fmt.Fprintf(&src, "    s = s + waste%d[T]()\n", i)
	}
	src.WriteString("    let u = useful(x)\n    s\n}\n\nfn run(n: Int) -> Int { top(P { x: n, y: n }) }\n\nfn main() -> Unit! {}\n")
	return src.String()
}

func TestSpecializeBudgetIgnoresInstantiationsItDiscarded(t *testing.T) {
	p := specialized(t, discardedFanOut(300))
	if len(p.Diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none: every candidate over the limit was discarded", p.Diags)
	}
	got := instances(p)
	sort.Strings(got)
	if want := "main.top[main.P]"; !strings.Contains(strings.Join(got, " "), want) {
		t.Fatalf("instantiations = %v, want %s among them", got, want)
	}
	if !strings.Contains(strings.Join(got, " "), "main.useful[main.P]") {
		t.Errorf("useful was left shared because discarded instantiations used the budget: %v", got)
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

const wideRecordSrc = `
type Wide = {
    pub a Int
    pub b Int
    pub c Int
}

`

func TestSpecializeKeepsARecursionThroughAFunctionValueShared(t *testing.T) {
	p := specialized(t, wideRecordSrc+`
fn viaValue[T](x: T, n: Int) -> T {
    if n == 0 { x } else {
        let f = viaValue[T]
        f(x, n - 1)
    }
}

fn run(n: Int) -> Int { viaValue(Wide { a: n, b: n, c: n }, n).c }

fn main() -> Unit! {}
`)
	if got := instances(p); len(got) != 0 {
		t.Errorf("instantiations = %v, want the wide recursion left shared", got)
	}
}

func TestSpecializeKeepsARecursionThroughAClosureShared(t *testing.T) {
	p := specialized(t, wideRecordSrc+`
fn viaClosure[T: Copy](x: T, n: Int) -> T {
    if n == 0 { x } else {
        let next = () => viaClosure(x, n - 1)
        next()
    }
}

fn run(n: Int) -> Int { viaClosure(Wide { a: n, b: n, c: n }, n).c }

fn main() -> Unit! {}
`)
	if got := instances(p); len(got) != 0 {
		t.Errorf("instantiations = %v, want the wide recursion left shared", got)
	}
}

func TestSpecializeSharesInstantiationsThatDifferOnlyInLifetime(t *testing.T) {
	p := specialized(t, `
type Tag['a] = {
    pub n Int
    pub m Int
}

fn keep[T: Copy](x: T) -> T { x }

fn viaA['a](v: Tag['a]) -> Int { keep(v).n }

fn viaB['b](v: Tag['b]) -> Int { keep(v).n }

fn run(n: Int) -> Int {
    let v = Tag { n: n, m: 1 }
    viaA(v) + viaB(v)
}

fn main() -> Unit! {}
`)
	if got := instances(p); len(got) != 1 {
		t.Errorf("instantiations = %v, want one shared by both lifetimes", got)
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestSpecializeRefusesAHashHolderThatOwnsAString(t *testing.T) {
	p := specialized(t, `
type Plain = {
    pub x Int
    pub y Int
}

type Named = {
    pub label String
    pub n Int
}

fn fingerprint[T: Hash](a: &T) -> u64 { hash(a) }

fn run(n: Int) -> Bool {
    let a = fingerprint(&Plain { x: n, y: 1 })
    let b = fingerprint(&Named { label: "k", n: n })
    a == b
}

fn main() -> Unit! {}
`)
	got := strings.Join(instances(p), " ")
	if !strings.Contains(got, "main.fingerprint[main.Plain]") {
		t.Errorf("a direct record holder is not specialized: %s", got)
	}
	if strings.Contains(got, "main.fingerprint[main.Named]") {
		t.Errorf("a holder of a String has no unboxed form, so specializing it gains nothing: %s", got)
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}
