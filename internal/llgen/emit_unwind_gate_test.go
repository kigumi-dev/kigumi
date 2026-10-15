package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
)

const dropHookSrc = `
type Res = resource {
    pub id Int
}

fn Res.drop(move self) -> Unit {
    let seen = "drop ${self.id}"
}

fn scope() -> Unit {
    let a = Res { id: 1 }
    let s = "text"
    let b = Res { id: 2 }
}

fn main() -> Unit! {
    scope()
}
`

func TestUnwindFollowsDropHooksWithoutATaskExecutor(t *testing.T) {
	hooks := plannedProgram(t, dropHookSrc)
	out := llgen.EmitProgram(hooks, llgen.EmitOptions{Catch: true, Reachable: mir.Reachable(hooks)})
	if !out.Unwind {
		t.Fatal("a program with a drop hook and no executor must unwind")
	}
	if !strings.Contains(fnBody(t, out.IR, "main.scope"), "@rt_release_outer(") {
		t.Error("scope does not sweep after a faulting drop")
	}
	plain := plannedProgram(t, unwindSrc)
	if llgen.EmitProgram(plain, llgen.EmitOptions{Catch: true, Reachable: mir.Reachable(plain)}).Unwind {
		t.Error("a program with no drop hook and no executor pays for unwinding")
	}
}
