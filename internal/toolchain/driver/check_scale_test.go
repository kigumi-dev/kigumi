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
			if got := scaleRSS(t, manyLoops(4000, header), 8*time.Second, 500); got != "12000\n" {
				t.Fatalf("stdout %q", got)
			}
		})
	}
}

const asyncPrelude = "import task from std/task\n\nasync fn tick() -> Int {\n    1\n}\n\n"
const asyncRun = "\nlet ex = task.currentThreadExecutor()\nprint \"result ${ex.runToCompletion(work())}\"\n"

func TestRunAsyncDefersAcrossAwaitsStayCheap(t *testing.T) {
	var src strings.Builder
	src.WriteString(asyncPrelude + "async fn work() -> Int {\n    let mut total = 0\n")
	for i := range 400 {
		fmt.Fprintf(&src, "    let s%d = \"s%d\"\n    defer print \"d${s%d}\"\n    total = total + await tick()\n", i, i, i)
	}
	src.WriteString("    total\n}\n" + asyncRun)
	out := scaleRSS(t, src.String(), 9*time.Second, 600)
	if !strings.HasSuffix(out, "result 400\n") {
		t.Fatalf("stdout tail %q", out[max(0, len(out)-40):])
	}
}

func TestRunOwnedLocalsAcrossAwaitStayCheap(t *testing.T) {
	var src strings.Builder
	src.WriteString(asyncPrelude + "async fn work() -> Int {\n    let mut total = 0\n")
	for i := range 3000 {
		fmt.Fprintf(&src, "    let v%d = \"s%d\"\n", i, i)
	}
	src.WriteString("    total = total + await tick()\n")
	for i := range 3000 {
		fmt.Fprintf(&src, "    total = total + (v%d.len().toInt() || 0)\n", i)
	}
	src.WriteString("    total\n}\n" + asyncRun)
	if got := scaleRSS(t, src.String(), 9*time.Second, 900); got != "result 13891\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestRunDeepElseIfChainStaysCheap(t *testing.T) {
	var src strings.Builder
	src.WriteString("fn pick(x: Int) -> Int {\n    if x == 0 {\n        0\n    }")
	for i := 1; i < 1200; i++ {
		fmt.Fprintf(&src, " else if x == %d {\n        %d\n    }", i, i)
	}
	src.WriteString(" else {\n        -1\n    }\n}\nprint \"${pick(77)}\"\n")
	if got := runGeneratedVM(t, src.String(), 3*time.Second); got != "77\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestRunNestedGenericCallsStayCheap(t *testing.T) {
	const n = 800
	src := "fn wrap[T](x: T) -> Option[T] {\n    Some(x)\n}\n\nfn work() -> Int {\n    let v = " + strings.Repeat("wrap(", n) + "1" + strings.Repeat(")", n) + "\n    0\n}\nprint \"${work()}\"\n"
	if got := runGeneratedVM(t, src, 3*time.Second); got != "0\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestRunOwnedLocalEachAcrossItsOwnAwaitStaysCheap(t *testing.T) {
	var src strings.Builder
	src.WriteString(asyncPrelude + "async fn work() -> Int {\n    let mut total = 0\n")
	for i := range 3000 {
		fmt.Fprintf(&src, "    let v%d = \"s%d\"\n    total = total + await tick()\n", i, i)
	}
	for i := range 3000 {
		fmt.Fprintf(&src, "    total = total + (v%d.len().toInt() || 0)\n", i)
	}
	src.WriteString("    total\n}\n" + asyncRun)
	if got := scaleRSS(t, src.String(), 10*time.Second, 1100); got != "result 16890\n" {
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
			if got := scaleRSS(t, repeatedBody(4000, c.body, ""), 8*time.Second, 500); got != c.want {
				t.Fatalf("stdout %q", got)
			}
		})
	}
}

func TestRunElseIfOptionChainStaysCheap(t *testing.T) {
	var src strings.Builder
	src.WriteString("fn pick(x: Int?) -> Int {\n    if x is Some(0) {\n        0\n    }")
	for i := 1; i < 1200; i++ {
		fmt.Fprintf(&src, " else if x is Some(%d) {\n        %d\n    }", i, i)
	}
	src.WriteString(" else {\n        -1\n    }\n}\nprint \"${pick(Some(77))}\"\n")
	if got := scaleRSS(t, src.String(), 6*time.Second, 400); got != "77\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestRunManyOwnedMovesStayCheap(t *testing.T) {
	var src strings.Builder
	src.WriteString("fn eat(s: String) -> Int {\n    s.len().toInt() || 0\n}\n\nfn work() -> Int {\n    let mut x = 0\n")
	for i := range 10000 {
		fmt.Fprintf(&src, "    let s%d = \"v%d\"\n    x = x + eat(s%d)\n", i, i, i)
	}
	src.WriteString("    x\n}\nprint \"${work()}\"\n")
	if got := runGeneratedVM(t, src.String(), 5*time.Second); got != "48890\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestRunManyGrowingArrayLoopsStayCheap(t *testing.T) {
	var src strings.Builder
	src.WriteString("fn work() -> Int {\n    let mut total = 0\n")
	for i := range 4000 {
		fmt.Fprintf(&src, "    let mut b%[1]d = Array.empty[String]()\n    let mut j%[1]d: usize = 0\n    for j%[1]d < 3 {\n        b%[1]d.push(\"x${j%[1]d}\")\n        j%[1]d = j%[1]d + 1\n    }\n    total = total + (b%[1]d.len().toInt() || 0)\n", i)
	}
	src.WriteString("    total\n}\nprint \"r ${work()}\"\n")
	if got := scaleRSS(t, src.String(), 15*time.Second, 700); got != "r 12000\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestRunManyOptionalOwnedPayloadsStayCheap(t *testing.T) {
	var src strings.Builder
	src.WriteString("fn g(i: Int) -> Option[Array[Int]] {\n    if i > 0 { Some(Array.of(i, 2)) } else { None }\n}\n\nfn work() -> Int {\n    let mut total = 0\n")
	for i := range 3000 {
		fmt.Fprintf(&src, "    if g(%[1]d) is Some(a%[1]d) {\n        total = total + (a%[1]d.len().toInt() || 0)\n    }\n", i)
	}
	src.WriteString("    total\n}\nprint \"r ${work()}\"\n")
	if got := scaleRSS(t, src.String(), 8*time.Second, 1000); got != "r 5998\n" {
		t.Fatalf("stdout %q", got)
	}
}
