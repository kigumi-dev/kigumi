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

func checkRecoverableOnly(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main), RecoverableOnly: true}, schema.GenerateDerive)
}

func TestRecoverableOnlyRejectsFatalConvenience(t *testing.T) {
	bad := checkRecoverableOnly(t, "let mut xs = Array.tryEmpty[Int]().unwrap()\nxs.push(1)\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "recoverable-only") {
		t.Fatalf("Array.push must be rejected under recoverable-only:\n%s", bad.Render())
	}
	good := checkRecoverableOnly(t, "let mut xs = Array.tryEmpty[Int]().unwrap()\nlet r = xs.tryPush(1)\n")
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

func TestRecoverableOnlyRejectsBoxNew(t *testing.T) {
	bad := checkRecoverableOnly(t, "import {Box} from std/alloc\n\nlet c = Box.new(1)\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "recoverable-only") {
		t.Fatalf("Box.new must be rejected under recoverable-only:\n%s", bad.Render())
	}
	good := checkRecoverableOnly(t, "import {Box} from std/alloc\n\nlet c = Box.tryNew(1)\n")
	if good.HasErrors() {
		t.Fatalf("Box.tryNew must pass under recoverable-only:\n%s", good.Render())
	}
}

func TestRecoverableOnlyAcceptsTrustedWrapper(t *testing.T) {
	good := checkRecoverableOnly(t, "let mut xs = Array.tryEmpty[Int]().unwrap()\n_ = xs.tryPush(1)\n_ = xs.tryReserve(4)\nlet n = xs.len()\n")
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
	bad := checkRecoverableOnly(t, "type P = { x String }\n\nlet p = P { x: \"s\" }\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "recoverable-only") {
		t.Fatalf("a record literal must be rejected under recoverable-only:\n%s", bad.Render())
	}
	ok := checkRecoverableOnly(t, "type P = { x Int }\n\nlet p = P { x: 1 }\nlet q = p.x\n")
	if ok.HasErrors() {
		t.Fatalf("a direct record literal must pass under recoverable-only:\n%s", ok.Render())
	}
}

func TestRecoverableOnlyAcceptsADirectVariant(t *testing.T) {
	ok := checkRecoverableOnly(t, "type Tick = Up(n: Int) | Flat\n\nfn pick(n: Int) -> Tick { if n > 0 { Up(n) } else { Flat } }\n\nlet r: Result[Int, Int] = Ok(if pick(1) is Up(v) { v } else { 0 })\nlet s = Some(2)\nlet n = s || 0\n")
	if ok.HasErrors() {
		t.Fatalf("a direct variant must pass under recoverable-only:\n%s", ok.Render())
	}
	bad := checkRecoverableOnly(t, "type Tag = Named(s: String) | Blank\n\nfn tag(s: String) -> Tag { Named(s) }\n\nlet t = tag(\"x\")\n")
	if !bad.HasErrors() || !strings.Contains(bad.Render(), "recoverable-only") {
		t.Fatalf("a variant with a carrier payload must be rejected:\n%s", bad.Render())
	}
}

// TestRecoverableOnlyJudgesMapKeyImpl runs sem alone: a planned build of Map.trySet with a derived record key stops in MIR.
func TestRecoverableOnlyJudgesMapKeyImpl(t *testing.T) {
	src := "import map from std/map\n\ntype Id = { pub n Int }\n\npure fn Id.hash(self) -> u64 {\n    let s = \"${self.n}\"\n    1\n}\n\n" +
		"if map.tryEmpty[Id, Int]() is Ok(m0) {\n    let mut m = m0\n    let r = m.trySet(Id { n: 1 }, 2)\n}\n"
	bad := checkRecoverableOnly(t, src)
	if want := "`trySet`, which calls `Id.hash` as `Hash.hash`"; !strings.Contains(bad.Render(), want) {
		t.Fatalf("an allocating key hash must be named through Map.trySet, want %q:\n%s", want, bad.Render())
	}
}
