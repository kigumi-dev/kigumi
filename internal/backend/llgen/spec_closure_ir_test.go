package llgen_test

import (
	"strings"
	"testing"
)

const closureProbe = `type Point = {
    pub x Int
    pub y Int
}

fn hold[T: Copy](x: T) -> T {
    let g = () => x
    g()
}

fn run(n: Int) -> Int {
    hold(Point { x: n, y: n + 1 }).y
}

print "${run(3)}"
`

func TestClosureOverARecordInAGenericBodyKeepsItInline(t *testing.T) {
	ir := specializedIR(t, closureProbe)
	hold := bodyOf(t, ir, `define %"kg.agg.main.Point" @"main.hold[main.Point]"(%"kg.agg.main.Point" %p0)`)
	clo := bodyOf(t, ir, `define %"kg.agg.main.Point" @"main.hold.closure1[main.Point]"(ptr %env)`)
	for what, body := range map[string]string{"main.hold[main.Point]": hold, "main.hold.closure1[main.Point]": clo} {
		for _, bad := range []string{`@"kg.box.`, `@"kg.unbox.`, "@rt_record(", "@rt_closure(", "@rt_callv("} {
			if strings.Contains(body, bad) {
				t.Errorf("%s calls %s:\n%s", what, bad, body)
			}
		}
	}
	if !strings.Contains(hold, `call %"kg.agg.main.Point" @"main.hold.closure1[main.Point]"(ptr `) {
		t.Errorf("the body does not call the closure by name:\n%s", hold)
	}
	if strings.Contains(ir, `@"main.hold.closure1"(`) {
		t.Error("the shared closure body is emitted although nothing calls it")
	}
}
