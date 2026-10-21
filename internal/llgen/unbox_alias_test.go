package llgen_test

import (
	"regexp"
	"strings"
	"testing"
)

const scalarBoxesSrc = `
import task from std/task

async fn twice(n: Int) -> Int {
    n * 2
}

fn work(xs: Array[Int]) -> Int {
    let mut total = 0
    let ex = task.currentThreadExecutor()
    for i in xs {
        total = total + ex.runToCompletion(twice(i))
    }
    total
}

fn main() -> Void! {}
`

func TestNativeScalarReleasesTheBoxesItReplaces(t *testing.T) {
	body := fnBody(t, emitIR(t, scalarBoxesSrc), "main.work")
	if !strings.Contains(body, "@rt_iter_at_int(") || strings.Contains(body, "@rt_builtin(") {
		t.Fatalf("the loop variable should be read into a register without an element box:\n%s", body)
	}
	boxed := regexp.MustCompile(`(%v\d+) = call ptr @rt_int\(i64 %v\d+, i32 \d+\)\n\s+store ptr %v\d+, ptr (%l\d+)`).FindStringSubmatch(body)
	if boxed == nil {
		t.Fatalf("no box made for the Future argument:\n%s", body)
	}
	arg := regexp.MustCompile(`(%v\d+) = load ptr, ptr ` + boxed[2] + `\n`).FindStringSubmatch(body)
	rest := body[strings.Index(body, "@rt_future_new("):]
	if arg == nil || !strings.Contains(rest[:min(len(rest), 400)], "@rt_release(ptr "+arg[1]+")") {
		t.Fatalf("the box made for the Future argument is not released after the Future takes it:\n%s", body)
	}
}

func TestTypeParameterCopyAsksTheRuntime(t *testing.T) {
	body := bodyOf(t, emitIR(t, `
fn repeat[T: Copy](value: T, n: Int) -> Array[T] {
    let mut out: Array[T] = Array.empty()
    let mut i = 0
    for i < n {
        out.push(value)
        i = i + 1
    }
    out
}

fn main() -> Void! {}
`), `define ptr @"main.repeat"(`)
	if !strings.Contains(body, "@rt_copy_mut(") || strings.Contains(body, "@rt_copy(") {
		t.Fatalf("the pushed type parameter should go through rt_copy_mut alone:\n%s", body)
	}
}

func TestNativeScalarBorrowKeepsItsBoxUntilTheCall(t *testing.T) {
	body := fnBody(t, emitIR(t, `
fn same[T: Eq](a: &T, b: &T) -> Bool {
    a.equals(b)
}

fn main() -> Void! {
    let z: f64 = 0.0
    let k = z / z
    print "${same(k, k)}"
}
`), "main.main")
	box := strings.Index(body, "@rt_float(")
	call := strings.Index(body, `@"main.same`)
	if box < 0 || call < box {
		t.Fatalf("no boxed borrow passed to same:\n%s", body)
	}
	if strings.Contains(body[box:call], "@rt_release(") {
		t.Fatalf("a box behind a borrow is released before the call reads it:\n%s", body)
	}
}
