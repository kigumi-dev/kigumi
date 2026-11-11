package driver_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// buildZigFloatObject compiles the runtime's Zig float module into an object
// the C harnesses link beside their C sources, exactly as buildOnce does.
// The module is freestanding Zig source, so a toolchain without zig cannot
// build it and the caller skips.
func buildZigFloatObject(t *testing.T, dir string) string {
	t.Helper()
	zig, err := testCompiler().LookTool("zig")
	if err != nil {
		t.Skip("needs zig to compile the Zig float runtime")
	}
	src, err := filepath.Abs("../../backend/llgen/runtimegen/runtime/zig/rt_float.zig")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "rt_float.o")
	args := []string{"build-obj", "-OReleaseSafe", "-fno-compiler-rt", "-fno-stack-check", "-fstrip", "-fPIC", "-femit-bin=" + out, src}
	if out, err := exec.Command(zig, args...).CombinedOutput(); err != nil {
		t.Fatalf("build the Zig float runtime: %v\n%s", err, out)
	}
	return out
}
