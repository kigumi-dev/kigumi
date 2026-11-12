package driver_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// bitsets fits live sets kept as n bits per block: quadratic, with a constant far below the maps and copies it replaced.
var bitsets = growthLimit{bytes: 3.3, insts: 2.2}

const (
	arrayDecl = "    let mut b%[1]d = Array.empty[String]()\n"
	arrayLoop = "    let mut j%[1]d: usize = 0\n    for j%[1]d < 3 {\n        b%[1]d.push(\"x${j%[1]d}\")\n        j%[1]d = j%[1]d + 1\n    }\n    total = total + (b%[1]d.len().toInt() || 0)\n"
)

func arrayLoops(n int, far bool) string {
	var src strings.Builder
	src.WriteString("fn work() -> Int {\n    let mut total = 0\n")
	for i := range n {
		if far {
			fmt.Fprintf(&src, arrayDecl, i)
		}
	}
	for i := range n {
		if !far {
			fmt.Fprintf(&src, arrayDecl, i)
		}
		fmt.Fprintf(&src, arrayLoop, i)
	}
	src.WriteString("    total\n}\nprint \"r ${work()}\"\n")
	return src.String()
}

func growingArrays(n int) string { return arrayLoops(n, false) }

func TestRunManyGrowingArrayLoopsStayCheap(t *testing.T) {
	requireGrowth(t, growingArrays, 4000, bitsets)
	if got := scaleRSS(t, growingArrays(4000), 150*time.Second, 800); got != "r 12000\n" {
		t.Fatalf("stdout %q", got)
	}
}

func farArrays(n int) string { return arrayLoops(n, true) }

// Every array is live from its declaration to its loop, so the live bitsets are dense and quadratic.
func TestRunArraysBorrowedFarFromTheirDeclarationsStayCheap(t *testing.T) {
	requireGrowth(t, farArrays, 4000, growthLimit{bytes: 3.0, insts: 2.2})
	if got := scaleRSS(t, farArrays(2400), 20*time.Second, 600); got != "r 7200\n" {
		t.Fatalf("stdout %q", got)
	}
}

func optionalPayloads(n int) string {
	var src strings.Builder
	src.WriteString("fn g(i: Int) -> Option[Array[Int]] {\n    if i > 0 { Some(Array.of(i, 2)) } else { None }\n}\n\nfn work() -> Int {\n    let mut total = 0\n")
	for i := range n {
		fmt.Fprintf(&src, "    if g(%[1]d) is Some(a%[1]d) {\n        total = total + (a%[1]d.len().toInt() || 0)\n    }\n", i)
	}
	src.WriteString("    total\n}\nprint \"r ${work()}\"\n")
	return src.String()
}

func TestRunManyOptionalOwnedPayloadsStayCheap(t *testing.T) {
	requireGrowth(t, optionalPayloads, 3000, bitsets)
	if got := scaleRSS(t, optionalPayloads(3000), 80*time.Second, 1000); got != "r 5998\n" {
		t.Fatalf("stdout %q", got)
	}
}

const asyncPrelude = "import task from std/task\n\nasync fn tick() -> Int {\n    1\n}\n\n"
const asyncRun = "\nlet ex = task.currentThreadExecutor()\nprint \"result ${ex.runToCompletion(work())}\"\n"

func asyncWork(n int, each bool) string {
	var src strings.Builder
	src.WriteString(asyncPrelude + "async fn work() -> Int {\n    let mut total = 0\n")
	for i := range n {
		fmt.Fprintf(&src, "    let v%d = \"s%d\"\n", i, i)
		if each {
			src.WriteString("    total = total + await tick()\n")
		}
	}
	if !each {
		src.WriteString("    total = total + await tick()\n")
	}
	for i := range n {
		fmt.Fprintf(&src, "    total = total + (v%d.len().toInt() || 0)\n", i)
	}
	src.WriteString("    total\n}\n" + asyncRun)
	return src.String()
}

func TestRunOwnedLocalsAcrossAwaitStayCheap(t *testing.T) {
	gen := func(n int) string { return asyncWork(n, false) }
	requireGrowth(t, gen, 3000, bitsets)
	if got := scaleRSS(t, gen(3000), 90*time.Second, 900); got != "result 13891\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestRunOwnedLocalEachAcrossItsOwnAwaitStaysCheap(t *testing.T) {
	gen := func(n int) string { return asyncWork(n, true) }
	requireGrowth(t, gen, 3000, bitsets)
	if got := scaleRSS(t, gen(3000), 100*time.Second, 1100); got != "result 16890\n" {
		t.Fatalf("stdout %q", got)
	}
}

func asyncDefers(n int) string {
	var src strings.Builder
	src.WriteString(asyncPrelude + "async fn work() -> Int {\n    let mut total = 0\n")
	for i := range n {
		fmt.Fprintf(&src, "    let s%d = \"s%d\"\n    defer print \"d${s%d}\"\n    total = total + await tick()\n", i, i, i)
	}
	src.WriteString("    total\n}\n" + asyncRun)
	return src.String()
}

// Every cancel edge runs the defers before it, so the MIR is quadratic; the bound keeps the bytes to that.
func TestRunAsyncDefersAcrossAwaitsStayCheap(t *testing.T) {
	requireGrowth(t, asyncDefers, 400, growthLimit{bytes: 4.4})
	out := scaleRSS(t, asyncDefers(400), 90*time.Second, 600)
	if !strings.HasSuffix(out, "result 400\n") {
		t.Fatalf("stdout tail %q", out[max(0, len(out)-40):])
	}
}
