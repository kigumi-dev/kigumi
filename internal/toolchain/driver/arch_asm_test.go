package driver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// TestArchCpuidAndRdtsc runs the x86-64 inline-assembly primitives natively;
// the VM cannot execute asm and the other architectures have stubs.
func TestArchCpuidAndRdtsc(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("std/arch's inline assembly is asserted for x86-64 Linux only")
	}
	src := `import arch from std/arch

fn main() {
    let c = arch.cpuid(0, 0)
    print "cpuid=${c.ebx > 0} ${c.edx > 0} ${c.ecx > 0}"
    print "tsc=${arch.rdtsc() > 0}"
    arch.pause()
}
`
	exe := buildProgram(t, src)
	out, err := exec.Command(exe).Output()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := string(out), "cpuid=true true true\ntsc=true\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestArchPrivilegedPrimitivesCompile checks that the privileged asm bodies
// lower and link; they cannot be executed in user space.
func TestArchPrivilegedPrimitivesCompile(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("std/arch's privileged primitives are assembled for x86-64 Linux only")
	}
	src := `import arch from std/arch

fn usePrivileged() {
    unsafe {
        let _ = arch.rdmsr(0x10)
        arch.wrmsr(0x10, 0)
        arch.invlpg(0)
        let _ = arch.inb(0)
        arch.outb(0, 0)
        let _ = arch.rdrand()
        let _ = arch.readCr(3)
        arch.writeCr(3, 0)
    }
}

fn main() {
    if false {
        usePrivileged()
    }
}
`
	if exe := buildProgram(t, src); exe == "" {
		t.Fatal("privileged primitive build produced no executable")
	}
}

// TestArchAvailableOnFreestanding keeps std/arch importable on a target that
// has no OS, and under --sys none, since it depends only on inline assembly.
func TestArchAvailableOnFreestanding(t *testing.T) {
	t.Parallel()
	std, _ := filepath.Abs("../../../std")
	for _, tc := range []struct {
		name, target, sys string
	}{
		{"uefi", "x86_64-uefi", ""},
		{"sys-none", "x86_64-linux-musl", "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"archgate\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(root, "main.kg"), []byte("import arch from std/arch\n\nfn main() {\n    arch.pause()\n}\n"), 0o644)
			target, err := buildconfig.ParseTarget(testCompiler(), tc.target, tc.sys)
			if err != nil {
				t.Fatal(err)
			}
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
			if err != nil {
				t.Fatal(err)
			}
			res, err := driver.Check(m)
			if err != nil {
				t.Fatal(err)
			}
			if res.HasErrors() {
				t.Fatalf("std/arch should be available on %s:\n%s", tc.target, res.Render())
			}
		})
	}
}
