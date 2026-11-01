package driver_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func scaleRSS(t *testing.T, src string, cpuLimit time.Duration, maxMB int64) string {
	t.Helper()
	out, rss, cpu := probedUsage(t, func() string { return runGeneratedVM(t, src, 10*time.Minute) })
	if cpu > cpuLimit {
		t.Fatalf("cpu time %v, limit %v", cpu, cpuLimit)
	}
	if rss>>10 > maxMB {
		t.Fatalf("peak rss %d MB, limit %d MB", rss>>10, maxMB)
	}
	return out
}

// linear fits per-block analysis state that grows with the block count, not with blocks times locals.
var linear = growthLimit{bytes: 2.5, insts: 2.2}

func manyLoops(n int, header string) string {
	var src strings.Builder
	src.WriteString("fn work(xs: &Array[Int]) -> Int {\n    let mut total = 0\n")
	for range n {
		fmt.Fprintf(&src, "    for %s {\n        total = total + 1\n    }\n", header)
	}
	src.WriteString("    total\n}\nlet xs = Array.of(1, 2, 3)\nprint \"${work(&xs)}\"\n")
	return src.String()
}

func TestRunFunctionWithManyLoopsStaysCheap(t *testing.T) {
	for _, header := range []string{"i in 0..3", "x in xs"} {
		t.Run(header, func(t *testing.T) {
			gen := func(n int) string { return manyLoops(n, header) }
			requireGrowth(t, gen, 4000, linear)
			if got := scaleRSS(t, gen(4000), 80*time.Second, 500); got != "12000\n" {
				t.Fatalf("stdout %q", got)
			}
		})
	}
}

func elseIfChain(n int, param, test, arg string) string {
	var src strings.Builder
	fmt.Fprintf(&src, "fn pick(x: %s) -> Int {\n    if %s {\n        0\n    }", param, fmt.Sprintf(test, 0))
	for i := 1; i < n; i++ {
		fmt.Fprintf(&src, " else if %s {\n        %d\n    }", fmt.Sprintf(test, i), i)
	}
	fmt.Fprintf(&src, " else {\n        -1\n    }\n}\nprint \"${pick(%s)}\"\n", arg)
	return src.String()
}

func TestRunDeepElseIfChainStaysCheap(t *testing.T) {
	gen := func(n int) string { return elseIfChain(n, "Int", "x == %d", "77") }
	requireGrowth(t, gen, 1200, linear)
	if got := runGeneratedVM(t, gen(1200), 30*time.Second); got != "77\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestRunElseIfOptionChainStaysCheap(t *testing.T) {
	gen := func(n int) string { return elseIfChain(n, "Int?", "x is Some(%d)", "Some(77)") }
	requireGrowth(t, gen, 1200, linear)
	if got := scaleRSS(t, gen(1200), 60*time.Second, 400); got != "77\n" {
		t.Fatalf("stdout %q", got)
	}
}

func nestedWraps(n int) string {
	return "fn wrap[T](x: T) -> Option[T] {\n    Some(x)\n}\n\nfn work() -> Int {\n    let v = " + strings.Repeat("wrap(", n) + "1" + strings.Repeat(")", n) + "\n    0\n}\nprint \"${work()}\"\n"
}

// The types nest n deep, so walking each local's type makes the bytes quadratic however the resolutions are cached.
func TestRunNestedGenericCallsStayCheap(t *testing.T) {
	requireGrowth(t, nestedWraps, 800, growthLimit{misses: 2.5, insts: 2.2})
	if got := runGeneratedVM(t, nestedWraps(800), 30*time.Second); got != "0\n" {
		t.Fatalf("stdout %q", got)
	}
}

func repeatedBody(n int, body, tail string) string {
	return "fn work() -> Int {\n    let mut total = 0\n" + strings.Repeat(body, n) + "    total\n}\nprint \"${work()}\"\n" + tail
}

func TestRunManyLoopShapesStayCheap(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"break":  {"    for i in 0..3 {\n        if i == 5 { break }\n        if i == 7 { continue }\n        total = total + 1\n    }\n", "12000\n"},
		"nested": {"    for a in 0..2 {\n        for b in 0..2 {\n            total = total + a + b\n        }\n    }\n", "16000\n"},
		"string": {"    for i in 0..3 {\n        let s = \"a${i}\"\n        total = total + (s.len().toInt() || 0)\n    }\n", "24000\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			gen := func(n int) string { return repeatedBody(n, c.body, "") }
			requireGrowth(t, gen, 4000, linear)
			if got := scaleRSS(t, gen(4000), 80*time.Second, 500); got != c.want {
				t.Fatalf("stdout %q", got)
			}
		})
	}
}

func ownedMoves(n int) string {
	var src strings.Builder
	src.WriteString("fn eat(s: String) -> Int {\n    s.len().toInt() || 0\n}\n\nfn work() -> Int {\n    let mut x = 0\n")
	for i := range n {
		fmt.Fprintf(&src, "    let s%d = \"v%d\"\n    x = x + eat(s%d)\n", i, i, i)
	}
	src.WriteString("    x\n}\nprint \"${work()}\"\n")
	return src.String()
}

func TestRunManyOwnedMovesStayCheap(t *testing.T) {
	requireGrowth(t, ownedMoves, 10000, linear)
	if got := runGeneratedVM(t, ownedMoves(10000), 50*time.Second); got != "48890\n" {
		t.Fatalf("stdout %q", got)
	}
}
