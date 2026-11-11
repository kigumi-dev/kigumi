package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const simdParitySrc = `
import simd from std/simd

fn main() {
    let mut a = simd.Vec.splat[i32, 4](7)
    a.setLane(2, 9)
    let b = simd.Vec.splat[i32, 4](3)
    let c = a.add(b).mul(b).sub(a)
    print "${c.lane(0)},${c.lane(1)},${c.lane(2)},${c.lane(3)}"

    let p = simd.Vec.splat[f64, 2](1.5)
    let q = simd.Vec.splat[f64, 2](2.0)
    let r = p.add(q).mul(q)
    print "${r.lane(0)} ${r.lane(1)}"

    let wrapped = simd.Vec.splat[u8, 16](255).add(simd.Vec.splat[u8, 16](1))
    print "${wrapped.lane(0)} ${wrapped.lane(15)}"

    let lanes = simd.Vec.fromArray(FixedArray.filled[u32, 4](5)).mul(simd.Vec.splat[u32, 4](3))
    print "${lanes.toArray()[0]} ${lanes.toArray()[3]}"
}
`

const simdParityWant = `23,23,27,23
7 7
0 0
15 15
`

// TestSimdVmNativeParity runs one Vec program through the VM and the native
// backend; the lane arithmetic must agree exactly on both.
func TestSimdVmNativeParity(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "main", "main.kg")
	if err := os.WriteFile(source, []byte(simdParitySrc), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &out, &errOut, []string{"main/main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("vm run: err %v exit %d\nstderr:\n%s", err, code, errOut.String())
	}
	if out.String() != simdParityWant {
		t.Fatalf("vm output\n--- got ---\n%s--- want ---\n%s", out.String(), simdParityWant)
	}
	if testCompiler().CCompiler() == nil {
		t.Log("native engine skipped: no C compiler")
		return
	}
	exe := filepath.Join(root, "simd-parity")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("native build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = root
	var nativeOut, nativeErr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &nativeOut, &nativeErr
	if err := cmd.Run(); err != nil {
		t.Fatalf("native run: %v\nstderr:\n%s", err, nativeErr.String())
	}
	if nativeOut.String() != simdParityWant {
		t.Fatalf("native output\n--- got ---\n%s--- want ---\n%s", nativeOut.String(), simdParityWant)
	}
}
