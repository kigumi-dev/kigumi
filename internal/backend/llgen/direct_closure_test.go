package llgen_test

import (
	"strings"
	"testing"
)

const closureLoopSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn run(n: Int) -> Int {
    let base = 10
    let on = true
    let p = Point { x: 1, y: 2 }
    let o: Int? = Some(3)
    let mut total = 0
    let mut i = 0
    for i < n {
        let f = (x: Int) => if on { x + base + p.y + (o || 0) } else { x }
        total = total + f(i)
        i = i + 1
    }
    total
}

fn main() -> Void! {}
`

func TestDirectClosureLoopAllocatesNothing(t *testing.T) {
	ir := emitIR(t, closureLoopSrc)
	body := fnBody(t, ir, "main.run")
	checkNoHeap(t, "main.run", body)
	if strings.Contains(body, "@rt_callv(") || !strings.Contains(body, `call ptr @"main.run.closure1"(ptr %l`) {
		t.Errorf("the closure must be called directly on its environment:\n%s", body)
	}
	if !strings.Contains(ir, `%"kg.clo.main.run.closure1" = type { i1, i64, %"kg.agg.main.Point", %"kg.adt.Option[Int]" }`) {
		t.Errorf("the environment is not laid out inline:\n%s", ir)
	}
}

func TestDirectClosureBodyTakesItsEnvironment(t *testing.T) {
	ir := emitIR(t, closureLoopSrc)
	body := bodyOf(t, ir, `define ptr @"main.run.closure1"(ptr %env, ptr %p4)`)
	if !strings.Contains(body, `getelementptr %"kg.clo.main.run.closure1", ptr %env, i32 0, i32 2`) {
		t.Errorf("the body does not read the Point capture in place:\n%s", body)
	}
	ir = emitIR(t, closureEscapeSrc)
	adapter := bodyOf(t, ir, `define ptr @"main.run.closure1$adapter"(`)
	if !strings.Contains(adapter, "@rt_at(ptr %env") || !strings.Contains(adapter, `@"main.run.closure1$boxed"(ptr `) {
		t.Errorf("the adapter must read the environment and call the boxed entry:\n%s", adapter)
	}
	entry := bodyOf(t, ir, `define ptr @"main.run.closure1$boxed"(`)
	if !strings.Contains(entry, `%"kg.clo.main.run.closure1"`) || !strings.Contains(entry, `@"main.run.closure1"(ptr `) {
		t.Errorf("the entry must rebuild the environment and call the body:\n%s", entry)
	}
}

// The adapter exists only once the closure escapes into a fn value.
const closureEscapeSrc = `
type Point = { pub x Int, pub y Int }

fn apply(g: fn(Int) -> Int, v: Int) -> Int { g(v) }

fn run(n: Int, base: Int, p: Point, on: Bool, o: Int?) -> Int {
    let f = (x: Int) => if on { x + base + p.y + (o || 0) } else { x }
    apply(f, n)
}

fn main() -> Void! {}
`
