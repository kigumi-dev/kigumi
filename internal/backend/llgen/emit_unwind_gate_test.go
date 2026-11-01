package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	mirreach "kigumi/internal/compiler/mir/reach"
)

const dropHookSrc = `
type Res = resource {
    pub id Int
}

fn Res.drop(move self) {
    let seen = "drop ${self.id}"
}

fn scope() {
    let a = Res { id: 1 }
    let s = "text"
    let b = Res { id: 2 }
}

fn main() -> Void! {
    scope()
}
`

func TestUnwindFollowsDropHooksWithoutATaskExecutor(t *testing.T) {
	hooks := plannedProgram(t, dropHookSrc)
	out := llgen.EmitProgram(hooks, llgen.EmitOptions{Catch: true, Reachable: mirreach.Reachable(hooks.R, hooks.Entry, hooks.Funcs, hooks.ByEnt)})
	if !out.Unwind {
		t.Fatal("a program with a drop hook and no executor must unwind")
	}
	if !strings.Contains(fnBody(t, out.IR, "main.scope"), "@rt_release_outer(") {
		t.Error("scope does not sweep after a faulting drop")
	}
	plain := plannedProgram(t, unwindSrc)
	if llgen.EmitProgram(plain, llgen.EmitOptions{Catch: true, Reachable: mirreach.Reachable(plain.R, plain.Entry, plain.Funcs, plain.ByEnt)}).Unwind {
		t.Error("a program with no drop hook and no executor pays for unwinding")
	}
}
