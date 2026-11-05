package driver_test

import (
	"os/exec"
	"runtime"
	"testing"
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
