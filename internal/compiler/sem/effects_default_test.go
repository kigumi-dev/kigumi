package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func checkNoDefault(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main), NoDefaultAllocator: true}, schema.GenerateDerive)
}

// TestNoDefaultAllocator checks the `allocator = none` profile.
func TestNoDefaultAllocator(t *testing.T) {
	ok := checkNoDefault(t, "noalloc fn add(a: Int, b: Int) -> Int {\n    a + b\n}\n\nlet n = add(1, 2)\n")
	if ok.HasErrors() {
		t.Fatalf("a noalloc entry must pass:\n%s", ok.Render())
	}
	bad := checkNoDefault(t, "let xs = Array.of(1, 2)\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "needs the default allocator") {
		t.Fatalf("an allocating entry must be rejected:\n%s", bad.Render())
	}
	scoped := checkNoDefault(t, "import {AllocatorHandle} from std/alloc\n\nfn run(h: AllocatorHandle) -> usize {\n    allocator h.scope() {\n        Array.of(1, 2).len()\n    }\n}\n\nlet n = 1\n")
	if scoped.HasErrors() {
		t.Fatalf("check errors:\n%s", scoped.Render())
	}
	for id := 1; id < len(scoped.Entities); id++ {
		if e := scoped.Entities[id]; e.Kind == sem.EntFn && e.Name == "run" {
			if !scoped.Fn(sem.EntityID(id)).Summary.Default {
				t.Fatal("an allocator block must satisfy the default-allocator requirement")
			}
			return
		}
	}
	t.Fatal("run not found")
}
