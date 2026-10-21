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

fn main() -> Void! {}
`

func TestCatchBuildsRegisterFrameRecords(t *testing.T) {
	p := plannedProgram(t, unwindSrc)
	ir := llgen.EmitWith(p, llgen.EmitOptions{Catch: true})
	body := fnBody(t, ir, "main.owner")
	for _, want := range []string{"%urec = alloca", "%umask = getelementptr", "@rt_unwind_enter(", "@rt_unwind_leave("} {
		if !strings.Contains(body, want) {
			t.Errorf("owner lacks %q:\n%s", want, body)
		}
	}
	if strings.Index(body, "@rt_unwind_leave(") > strings.LastIndex(body, "\n  ret ") {
		t.Errorf("the record is left after the return:\n%s", body)
	}
	for _, name := range []string{"main.plain", "main.tick"} {
		if strings.Contains(fnBody(t, ir, name), "@rt_unwind_enter(") {
			t.Errorf("%s registers a frame record", name)
		}
	}
}

func TestCatchOffEmitsNoFrameRecords(t *testing.T) {
	ir := llgen.EmitWith(plannedProgram(t, unwindSrc), llgen.EmitOptions{})
	if strings.Contains(fnBody(t, ir, "main.owner"), "rt_unwind") {
		t.Error("a build without task catch registers frame records")
	}
}

const unwindTempsSrc = `
type Res = resource {
    pub id Int
}

fn Res.drop(move self) {
    let seen = "drop ${self.id}"
}

fn make(n: Int) -> Res {
    Res { id: n }
}

fn consume(a: Res, n: Int) -> Int {
    n
}

fn explode() -> Int {
    None.expect("explode")
}

async fn tick() -> Int {
    1
}

async fn boom() -> Int {
    let v = consume(make(1), explode() + (await tick()))
    v
}

fn scope() {
    let a = make(1)
    let s = "text"
    let b = make(2)
}

fn main() -> Void! {}
`

func TestAsyncTemporariesRegisterARecordOverTheFrame(t *testing.T) {
	ir := llgen.EmitWith(plannedProgram(t, unwindTempsSrc), llgen.EmitOptions{Catch: true})
	body := fnBody(t, ir, "main.boom")
	for _, want := range []string{"@rt_unwind_enter_slots(", "@rt_unwind_leave(", "%umask = alloca"} {
		if !strings.Contains(body, want) {
			t.Errorf("boom lacks %q:\n%s", want, body)
		}
	}
	if got := strings.Count(body, "@rt_unwind_leave("); got < 2 {
		t.Errorf("boom leaves its record %d time(s); a suspension and the return both must", got)
	}
}

func TestFaultingDropsOutsideATaskSweepTheFrame(t *testing.T) {
	ir := llgen.EmitWith(plannedProgram(t, unwindTempsSrc), llgen.EmitOptions{Catch: true})
	body := fnBody(t, ir, "main.scope")
	for _, want := range []string{"@rt_release_outer(", "@rt_sweep_release(", "@rt_outer_fatal("} {
		if !strings.Contains(body, want) {
			t.Errorf("scope lacks %q:\n%s", want, body)
		}
	}
	off := llgen.EmitWith(plannedProgram(t, unwindTempsSrc), llgen.EmitOptions{})
	if b := fnBody(t, off, "main.scope"); strings.Contains(b, "_outer") || strings.Contains(b, "rt_sweep") {
		t.Errorf("a build without task catch contains faulting drops:\n%s", b)
	}
}
