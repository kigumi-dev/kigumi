package driver_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func TestSimdShapeSpecializationVmNative(t *testing.T) {
	var fan strings.Builder
	fan.WriteString("import simd from std/simd\n")
	for i := 0; i < 9; i++ {
		fmt.Fprintf(&fan, "fn g%d[T: Copy, const N: usize](x: T, v: simd.Vec[i32, N]) -> Int { g%d((x, 1), v) + g%d((2, x), v) }\n", i, i+1, i+1)
	}
	fan.WriteString("fn g9[T: Copy, const N: usize](x: T, v: simd.Vec[i32, N]) -> Int { v.add(v).lane(0).toInt() }\n")
	fan.WriteString("print \"${g0((1, 2), simd.Vec.splat[i32, 4](1))}\"\n")
	var chain strings.Builder
	chain.WriteString("import simd from std/simd\n")
	for i := 0; i < 299; i++ {
		fmt.Fprintf(&chain, "fn g%d[const N: usize](v: simd.Vec[i32, N]) -> Int { g%d(v) }\n", i, i+1)
	}
	chain.WriteString("fn g299[const N: usize](v: simd.Vec[i32, N]) -> Int { v.add(v).lane(0).toInt() }\n")
	chain.WriteString("print \"${g0(simd.Vec.splat[i32, 4](1))}\"\n")
	for _, tc := range []struct{ name, src, out string }{
		{"fanout", fan.String(), "1024\n"},
		{"chain", chain.String(), "2\n"},
		{"callbacks", simdCallbackShapes, "9 13 7 9 2 2\n"},
		{"async", simdAsyncShapes, "drop 7\n2\ndrop 9\nparked\ncancel\ndefer\ndrop 11\n5\n"},
		{"lane type", simdLaneTypeShape, "6\n"},
	} {
		t.Run(tc.name, func(t *testing.T) { runSimdShapeProgram(t, tc.src, tc.out) })
	}
}

const simdLaneTypeShape = `import simd from std/simd
fn twice[T: Copy](v: simd.Vec[T, 4]) -> simd.Vec[T, 4] { v.add(v) }
let f = twice[i32]
print "${f(simd.Vec.splat[i32, 4](3)).lane(0)}"
`

func TestSimdShapeInClosureRejectsInvalidInstance(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{"main.kg": `import simd from std/simd
fn make[const N: usize]() -> Int {
    let f = () => simd.Vec.splat[i32, N](1).lane(0).toInt()
    f()
}

print "${make[3]()}"
`})
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	_, ok, err := driver.Compile(m, &stderr)
	if err != nil || ok || !strings.Contains(stderr.String(), "E976") {
		t.Fatalf("compile: ok=%v err=%v\n%s", ok, err, stderr.String())
	}
}

func TestSimdShapeKeepsNoallocCopyDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{"main.kg": `import simd from std/simd
noalloc fn strict[T: Copy, const N: usize](xs: &Array[T], v: simd.Vec[i32, N]) -> T? {
    _ = v.add(v)
    xs.last()
}
let v = simd.Vec.splat[i32, 4](1)
let _ = strict(&Array.of((1, 2)), v)
`})
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	_, ok, err := driver.Compile(m, &stderr)
	if err != nil || ok || !strings.Contains(stderr.String(), "E1700") {
		t.Fatalf("compile: ok=%v err=%v\n%s", ok, err, stderr.String())
	}
}

const simdCallbackShapes = `import simd from std/simd
interface Value { fn number(self) -> Int }
type A = { pub x Int }
type B = { pub x Int, pub y Int }
fn A.number(self) -> Int { self.x }
fn B.number(self) -> Int { self.x + self.y }
fn g[T: Value, const N: usize](x: T, v: simd.Vec[i32, N]) -> Int {
    x.number() + v.add(v).lane(0).toInt()
}
type Pair[T, const N: usize] = { pub value T, pub vector simd.Vec[i32, N] }
fn pair[T: Copy, const N: usize](x: T, v: simd.Vec[i32, N]) -> Pair[T, N] {
    Pair { value: x, vector: v.add(v) }
}
fn nested[const N: usize](v: simd.Vec[i32, N]) -> Int {
    let f = () => { let inner = () => v.add(v).lane(0).toInt(); inner() }
    f()
}
noalloc fn strict[T: Copy, const N: usize](x: T, v: simd.Vec[i32, N]) -> simd.Vec[i32, N] {
    v.add(v)
}
let v = simd.Vec.splat[i32, 4](1)
let p = pair((7, 9), v)
let (x, y) = p.value
print "${g(A { x: 7 }, v)} ${g(B { x: 5, y: 6 }, v)} ${x} ${y} ${p.vector.lane(0)} ${nested(strict((1, 2), v)) / 2}"
`

const simdAsyncShapes = `import simd from std/simd
import task from std/task
type Guard = resource { pub marker Int }
fn Guard.drop(move self) { print "drop ${self.marker}" }
async fn inner[T, const N: usize](x: T, v: simd.Vec[i32, N]) -> Int {
    await task.yieldNow()
    v.add(v).lane(0).toInt()
}
async fn relay[T, const N: usize](x: T, v: simd.Vec[i32, N]) -> Int {
    await inner(x, v)
}
async fn stall() -> Int { 0 }
async fn park() -> Int { await task.spawn(stall()) }
async fn parked[T, const N: usize](x: T, v: simd.Vec[i32, N]) -> Int {
    let doubled = v.add(v)
    defer print "defer"
    print "parked"
    let n = await park()
    doubled.lane(0).toInt() + n
}
async fn cancelRoot() -> Int {
    let w = task.spawn(parked(Guard { marker: 11 }, simd.Vec.splat[i32, 4](1)))
    let tick = task.spawn(stall())
    let _ = await tick
    w.cancel()
    print "cancel"
    5
}
let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(relay(Guard { marker: 7 }, simd.Vec.splat[i32, 4](1)))}"
_ = relay(Guard { marker: 9 }, simd.Vec.splat[i32, 4](1))
print "${ex.runToCompletion(cancelRoot())}"
`

func runSimdShapeProgram(t *testing.T, src, want string) {
	t.Helper()
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{"main.kg": src})
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &stdout, &stderr, nil)
	if err != nil || code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("VM: err=%v exit=%d stdout=%q stderr=%q, want %q", err, code, stdout.String(), stderr.String(), want)
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := filepath.Join(root, "simd")
	if ok, err := testBuild(m, exe, &stderr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, stderr.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe)
	cmd.Env = os.Environ()
	if countsNativeLive() {
		cmd.Env = append(cmd.Env, "KIGUMI_LIVE_REPORT=1")
	}
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("native: %v\n%s", err, stderr.String())
	}
	errText := stderr.String()
	if countsNativeLive() {
		if m := nativeLiveLine.FindStringSubmatch(errText); m == nil || m[1] != "0" {
			t.Fatalf("native live objects: %q", errText)
		}
		errText = nativeLiveLine.ReplaceAllString(errText, "")
	}
	if stdout.String() != want || errText != "" {
		t.Fatalf("native stdout=%q stderr=%q, want %q", stdout.String(), errText, want)
	}
}
