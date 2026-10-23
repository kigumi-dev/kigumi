package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

const keptTypeSrc = `
type Used = { n: Int }

type Unused = { n: Int }

fn build() -> Used { Used { n: 1 } }

let mut xs: Array[Used] = Array.empty()
xs.push(build())
print "${xs[0].n}"
`

func namedType(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for id := 1; id < len(r.Entities); id++ {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntType && e.Name == name {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("no type %s", name)
	return 0
}

func TestEmitKeepsOnlyReachableDescriptors(t *testing.T) {
	prog := plannedProgram(t, keptTypeSrc)
	rs := mir.Reachable(prog)
	ir := llgen.EmitWith(prog, llgen.EmitOptions{Reachable: rs})
	if !strings.Contains(ir, `@".type.main.Used" =`) {
		t.Fatalf("a constructed type has no descriptor:\n%s", ir)
	}
	if strings.Contains(ir, `@".type.main.Unused"`) {
		t.Fatalf("a type nothing reaches still has a descriptor:\n%s", ir)
	}
}

func TestEmitRefusesDescriptorOfUnkeptType(t *testing.T) {
	prog := plannedProgram(t, keptTypeSrc)
	rs := mir.Reachable(prog)
	delete(rs.Types, namedType(t, prog.R, "Used"))
	defer func() {
		got := recover()
		msg, _ := got.(string)
		if !strings.Contains(msg, "main.Used") || !strings.Contains(msg, "did not keep") {
			t.Fatalf("want a panic naming the unkept type, got %v", got)
		}
	}()
	llgen.EmitWith(prog, llgen.EmitOptions{Reachable: rs})
}
