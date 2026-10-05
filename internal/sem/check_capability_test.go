package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

// This package cannot import MIR to build a CapabilityUse, so the test builds one directly.
func TestCheckStrictCapabilitiesReportsE1370(t *testing.T) {
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte("let n = 1\n")))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.Check(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)})
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	var file sem.FileID
	for i := range res.Files {
		if res.Files[i].Tree == main.Entry {
			file = sem.FileID(i)
		}
	}
	res.CheckStrictCapabilities([]sem.CapabilityUse{{
		Op:   "HOST_SHELL",
		File: file,
		Node: main.Entry.Root,
		Path: []string{"main", "run", "HOST_SHELL"},
	}})
	if !res.HasErrors() {
		t.Fatal("expected CheckStrictCapabilities to report an error")
	}
	out := res.Render()
	if !strings.Contains(out, "E1370") || !strings.Contains(out, "HOST_SHELL") || !strings.Contains(out, "main -> run -> HOST_SHELL") {
		t.Fatalf("expected E1370 naming HOST_SHELL and its path, got:\n%s", out)
	}
}
