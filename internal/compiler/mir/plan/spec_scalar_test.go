package plan_test

import (
	"strings"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/devkit/mirfixture"
)

const specScalarSrc = `
fn wrap[T](x: T) -> T? { Some(x) }

fn firstOf[T: Copy](xs: &Array[T]) -> T? { xs.get(0) }

fn orElse[T](o: T?, d: T) -> T { o.unwrapOr(d) }

fn pair[T: Copy](x: T) -> (T, T) { (x, x) }

fn sink[T](x: T) -> Int {
    let mut out = Array.empty[T]()
    out.push(x)
    1
}

noalloc fn sinkNoalloc[T: Copy](x: T) -> Int { 1 }

fn keep[T: Copy](x: T) -> T { x }

fn run(n: Int) -> Int {
    let xs = Array.of(n, 2)
    let (a, b) = pair(n)
    let f = keep[Int]
    (wrap(n) || 0) + (firstOf(&xs) || 0) + orElse(Some(n), 1) + a + b + sink(n) + sinkNoalloc(n) + f(n)
}

fn main() -> Void! {}
`

// A scalar instance builds the aggregate it returns unboxed, and passes a returned scalar (or a noalloc
// generic's scalar parameter) in a register; a function value keeps the shared body.
func TestSpecializeAScalarInstanceThatGains(t *testing.T) {
	p := specialized(t, specScalarSrc)
	got := strings.Join(instances(p), " ")
	for _, want := range []string{"main.wrap[Int]", "main.firstOf[Int]", "main.pair[Int]", "main.orElse[Int]", "main.sinkNoalloc[Int]"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %s among %q", want, got)
		}
	}
	for _, shared := range []string{"main.sink[", "main.keep["} {
		if strings.Contains(got, shared) {
			t.Errorf("%s gains nothing from an instance, yet got one: %q", shared, got)
		}
	}
	wrap := mirfixture.FuncNamed(t, p, "main.wrap[Int]")
	if wrap.DSig.Ret == 0 || mirfixture.CountOp(wrap, midir.OpBoxDirect) != 0 {
		t.Errorf("wrap[Int] does not return its Option unboxed:\n%s", p.Dump())
	}
}
