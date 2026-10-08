package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/llgen"
)

const unwindSrc = `
fn noise(s: String) -> Int {
    s.len().toInt() || 0
}

fn owner(n: Int) -> Int {
    let s = "frame ${n}"
    let t = "again ${n}"
    noise(s) + noise(t)
}

fn plain() -> Int {
    3
}

async fn tick() -> Int {
    1
}

fn main() -> Unit! {}
`

func TestCatchBuildsRegisterFrameRecords(t *testing.T) {
	p := plannedProgram(t, unwindSrc)
	ir := llgen.EmitWith(p, llgen.EmitOptions{Catch: true})
	body := bodyOf(t, ir, `define ptr @"main.owner"(`)
	for _, want := range []string{"%uslots = alloca", "%umask = alloca", "@rt_unwind_enter(", "@rt_unwind_leave("} {
		if !strings.Contains(body, want) {
			t.Errorf("owner lacks %q:\n%s", want, body)
		}
	}
	if strings.Index(body, "@rt_unwind_leave(") > strings.LastIndex(body, "ret ptr") {
		t.Errorf("the record is left after the return:\n%s", body)
	}
	for _, name := range []string{`define ptr @"main.plain"(`, `define ptr @"main.tick"(`} {
		if strings.Contains(bodyOf(t, ir, name), "@rt_unwind_enter(") {
			t.Errorf("%s registers a frame record", name)
		}
	}
}

func TestCatchOffEmitsNoFrameRecords(t *testing.T) {
	ir := llgen.EmitWith(plannedProgram(t, unwindSrc), llgen.EmitOptions{})
	if strings.Contains(bodyOf(t, ir, `define ptr @"main.owner"(`), "rt_unwind") {
		t.Error("a build without task catch registers frame records")
	}
}
