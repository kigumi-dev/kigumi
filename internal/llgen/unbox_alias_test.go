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

fn work(n: Int) -> Int {
    let mut total = 0
    let ex = task.currentThreadExecutor()
    for i in 0..n {
        total = total + ex.runToCompletion(twice(i))
    }
    total
}

fn main() -> Unit! {}
`

func TestNativeScalarReleasesTheBoxesItReplaces(t *testing.T) {
	body := bodyOf(t, emitIR(t, scalarBoxesSrc), `define ptr @"main.work"(`)
	elem := regexp.MustCompile(`= call ptr @rt_builtin\([^\n]*\n\s+store ptr %v\d+, ptr %l\d+\n\s+(%v\d+) = load ptr, ptr %l\d+\n\s+%v\d+ = call i64 @rt_int_val\(ptr %v\d+\)\n([^\n]*)`).FindStringSubmatch(body)
	if elem == nil || !strings.Contains(elem[2], "@rt_release(ptr "+elem[1]+")") {
		t.Fatalf("the element box read into the loop variable is not released after unboxing:\n%s", body)
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

fn main() -> Unit! {}
`), `define ptr @"main.repeat"(`)
	if !strings.Contains(body, "@rt_copy_mut(") || strings.Contains(body, "@rt_copy(") {
		t.Fatalf("the pushed type parameter should go through rt_copy_mut alone:\n%s", body)
	}
}
