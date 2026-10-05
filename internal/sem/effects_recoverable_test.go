package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

func checkRecoverableOnly(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.Check(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main), RecoverableOnly: true})
}

func TestRecoverableOnlyRejectsFatalConvenience(t *testing.T) {
	bad := checkRecoverableOnly(t, "let mut xs = Array.empty[Int]()\nxs.push(1)\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "recoverable-only") {
		t.Fatalf("Array.push must be rejected under recoverable-only:\n%s", bad.Render())
	}
	good := checkRecoverableOnly(t, "let mut xs = Array.empty[Int]()\nlet r = xs.tryPush(1)\n")
	if good.HasErrors() {
		t.Fatalf("Array.tryPush must pass under recoverable-only:\n%s", good.Render())
	}
}

// TestRecoverableOnlyRejectsSharedNew checks the bodiless-primitive registry, not just the EdgeAlloc case.
func TestRecoverableOnlyRejectsSharedNew(t *testing.T) {
	bad := checkRecoverableOnly(t, "import {Shared} from std/alloc\n\nlet c = Shared.new(1)\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "recoverable-only") {
		t.Fatalf("Shared.new must be rejected under recoverable-only:\n%s", bad.Render())
	}
	good := checkRecoverableOnly(t, "import {Shared} from std/alloc\n\nlet c = Shared.tryNew(1)\n")
	if good.HasErrors() {
		t.Fatalf("Shared.tryNew must pass under recoverable-only:\n%s", good.Render())
	}
}

func TestRecoverableOnlyAcceptsTrustedWrapper(t *testing.T) {
	good := checkRecoverableOnly(t, "let mut xs = Array.empty[Int]()\n_ = xs.tryPush(1)\n_ = xs.tryReserve(4)\nlet n = xs.len()\n")
	if good.HasErrors() {
		t.Fatalf("Array.tryPush/tryReserve must pass under recoverable-only:\n%s", good.Render())
	}
}

func TestRecoverableOnlyAcceptsTrySecureErase(t *testing.T) {
	bad := checkRecoverableOnly(t, "import mem from std/mem\n\nlet mut b = Bytes.tryZeros(4).unwrap()\nmem.secureErase(&mut b)\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "recoverable-only") {
		t.Fatalf("mem.secureErase must be rejected under recoverable-only:\n%s", bad.Render())
	}
	good := checkRecoverableOnly(t, "import mem from std/mem\n\nlet mut b = Bytes.tryZeros(4).unwrap()\nlet r = mem.trySecureErase(&mut b)\n")
	if good.HasErrors() {
		t.Fatalf("mem.trySecureErase must pass under recoverable-only:\n%s", good.Render())
	}
}

// TestRecoverableOnlyRejectsCompilerGeneratedAlloc checks a record literal is rejected too, same as a bodiless fatal convenience.
func TestRecoverableOnlyRejectsCompilerGeneratedAlloc(t *testing.T) {
	bad := checkRecoverableOnly(t, "type P = { x Int }\n\nlet p = P { x: 1 }\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "recoverable-only") {
		t.Fatalf("a record literal must be rejected under recoverable-only:\n%s", bad.Render())
	}
}
