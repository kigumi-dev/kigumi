package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

func checkIsrArch(t *testing.T, arch, os, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main), Arch: arch, OS: os, IsrContracts: mir.IsrContracts}, schema.GenerateDerive)
}

func nakedIsr(body, extra string) string {
	return `
extern(C) {
    fn raw()
}

export(C) {
    pub fn wrapped() -> Int { 70000 }
    pub fn plain() {}
}

export(interrupt) {
    pub fn inner() {}
}

export(interrupt, naked) {
    pub fn isr() {
        asm {
` + body + `
        }
    }
}
` + extra
}

func TestInterruptNakedVerified(t *testing.T) {
	res := checkIsrArch(t, "amd64", "linux", nakedIsr(`            "push rbx"
            "xor ebx, ebx"
            "call {f}"
            "pop rbx"
            "ret"
            f: sym inner`, ""))
	if res.HasErrors() {
		t.Fatalf("a balanced, aligned naked body calling a checked handler must pass:\n%s", res.Render())
	}
}

func TestInterruptNakedRejected(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"extern sym", "\"push rbx\"\n\"call {f}\"\n\"pop rbx\"\n\"ret\"\nf: sym raw", "calls `raw`, which has no body the compiler can check and no reviewed interrupt contract"},
		{"export(C) wrapper", "\"jmp {f}\"\nf: sym wrapped", "names `wrapped`, whose export(C) wrapper may box its arguments"},
		{"misaligned call", "\"call {f}\"\n\"ret\"\nf: sym inner", "`call` runs with the stack misaligned"},
		{"callee-saved", "\"mov rbx, 1\"\n\"ret\"", "overwrites callee-saved `rbx` without pushing it first"},
		{"memory", "\"mov rax, [rdi]\"\n\"ret\"", "accesses memory, which may be MMIO"},
		{"unbalanced", "\"push rbx\"\n\"ret\"", "leaves the stack unbalanced"},
		{"fallthrough", "\"xor eax, eax\"", "must end in `ret` or a `jmp`"},
		{"indirect jump", "\"jmp rax\"", "not a checked `sym` operand"},
		{"divide", "\"div rcx\"\n\"ret\"", "`div rcx` cannot be classified"},
		{"lock", "\"lock add rax, 1\"\n\"ret\"", "lock or repeat prefix"},
		{"pop order", "\"push rbx\"\n\"push r12\"\n\"pop rbx\"\n\"pop r12\"\n\"ret\"", "does not restore the register pushed last"},
		{"cr hides call", "\"xor eax, eax # c\\rcall rax\"\n\"ret\"", "control character '\\r'"},
		{"cr hides syscall", "\"push rbx # c\\rsyscall\"\n\"pop rbx\"\n\"ret\"", "control character '\\r'"},
		{"block comment", "\"nop /* # */ xor eax, eax\"\n\"ret\"", "writes `/`"},
		{"literal sym placeholder", "\"push rbx\"\n\"call @sym\"\n\"pop rbx\"\n\"ret\"", "writes `@`"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := checkIsrArch(t, "amd64", "linux", nakedIsr(c.body, "")).Render()
			if !strings.Contains(out, "handler `isr`") || !strings.Contains(out, c.want) {
				t.Fatalf("want a rejection mentioning %q:\n%s", c.want, out)
			}
		})
	}
}

// A zero-argument Void export(C) wrapper boxes nothing, so a naked handler may reach it.
func TestInterruptNakedPlainExportC(t *testing.T) {
	res := checkIsrArch(t, "amd64", "linux", nakedIsr("\"jmp {f}\"\nf: sym plain", ""))
	if res.HasErrors() {
		t.Fatalf("jmp to a trivial export(C) fn must pass:\n%s", res.Render())
	}
}

func TestInterruptNakedUnsupportedTarget(t *testing.T) {
	for _, tc := range []struct{ arch, os, want string }{
		{"arm64", "linux", "asm for arm64 cannot be verified yet"},
		{"amd64", "windows", "Microsoft x64 ABI"},
	} {
		src := "export(interrupt, naked) {\n    pub fn isr() {\n        asm {\n            \"ret\"\n        }\n    }\n}\n"
		if out := checkIsrArch(t, tc.arch, tc.os, src).Render(); !strings.Contains(out, "E1089") || !strings.Contains(out, tc.want) {
			t.Fatalf("%s/%s must be rejected as unverifiable:\n%s", tc.arch, tc.os, out)
		}
	}
}

func TestInterruptInlineAsm(t *testing.T) {
	ok := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        unsafe {
            asm {
                "xor eax, eax"
                "nop"
                clobber rax
            }
        }
    }
}
`)
	if ok.HasErrors() {
		t.Fatalf("inline register arithmetic on a declared clobber must pass:\n%s", ok.Render())
	}
	for _, c := range []struct{ body, want string }{
		{"\"syscall\"", "`syscall` cannot be classified"},
		{"\"xor eax, eax\"", "writes `rax`, which the block neither clobbers nor outputs"},
		{"\"push rax\"\nclobber rax", "inline asm may not `push`"},
		{"\"nop # c\\rsyscall\"", "control character '\\r'"},
	} {
		out := checkIsr(t, "export(interrupt) {\n    pub fn tick() {\n        unsafe {\n            asm {\n"+c.body+"\n            }\n        }\n    }\n}\n").Render()
		if !strings.Contains(out, "E1089") || !strings.Contains(out, c.want) {
			t.Fatalf("want an E1089 mentioning %q:\n%s", c.want, out)
		}
	}
}
