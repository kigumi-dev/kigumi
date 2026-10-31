package driver_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func checkStrict(t *testing.T, main string, strict bool, budgetKiB int) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"budget\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(main), 0o644)
	std, _ := filepath.Abs("../../../std")
	target := driver.HostTarget()
	target.StrictStack = strict
	target.StackBudgetKiB = budgetKiB
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	return res.Render()
}

func TestStrictStackBounded(t *testing.T) {
	t.Parallel()
	out := checkStrict(t, "fn addOne(n: Int) -> Int {\n    n + 1\n}\n\nlet total = addOne(41)\nprint \"${total}\"\n", true, 0)
	if strings.Contains(out, "error") {
		t.Fatalf("expected a bounded chain to pass strict, got:\n%s", out)
	}
}

func TestStrictStackRecursive(t *testing.T) {
	t.Parallel()
	out := checkStrict(t, "fn fact(n: Int) -> Int {\n    if n <= 1 { 1 } else { n * fact(n - 1) }\n}\n\nlet total = fact(5)\nprint \"${total}\"\n", true, 0)
	if !strings.Contains(out, "E1410") || !strings.Contains(out, "unbounded") {
		t.Fatalf("expected E1410 stack-chain-unbounded, got:\n%s", out)
	}
}

func TestStrictStackMutualRecursionPathNotDuplicated(t *testing.T) {
	t.Parallel()
	src := "fn a(n: Int) -> Int {\n    if n <= 0 { 0 } else { b(n - 1) }\n}\n\nfn b(n: Int) -> Int {\n    if n <= 0 { 0 } else { a(n - 1) }\n}\n\nlet total = a(5)\nprint \"${total}\"\n"
	out := checkStrict(t, src, true, 0)
	if !strings.Contains(out, "E1410") {
		t.Fatalf("expected E1410 stack-chain-unbounded, got:\n%s", out)
	}
	if strings.Contains(out, "a -> a") || strings.Contains(out, "b -> b") {
		t.Fatalf("expected the cycle path to not repeat a name back to back, got:\n%s", out)
	}
}

func TestStrictStackDynamicDispatch(t *testing.T) {
	t.Parallel()
	src := "fn applyIt(f: fn(Int) -> Int, n: Int) -> Int {\n    f(n)\n}\n\nfn double(n: Int) -> Int {\n    n * 2\n}\n\nlet total = applyIt(double, 3)\nprint \"${total}\"\n"
	out := checkStrict(t, src, true, 0)
	if !strings.Contains(out, "E1411") {
		t.Fatalf("expected E1411 stack-chain-unknown, got:\n%s", out)
	}
}

func TestStrictStackBudgetExceeded(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("fn wide() -> Int {\n    let a0 = 0\n")
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&b, "    let a%d = a%d + 1\n", i, i-1)
	}
	b.WriteString("    a200\n}\n\nlet total = wide()\nprint \"${total}\"\n")
	out := checkStrict(t, b.String(), true, 1)
	if !strings.Contains(out, "E1412") || !strings.Contains(out, "stack budget") {
		t.Fatalf("expected E1412 stack-budget-exceeded with a 1 KiB budget, got:\n%s", out)
	}
}

// Carrier lease escape is checked regardless of --stack-profile.
func TestCarrierLeaseEscapeRejected(t *testing.T) {
	t.Parallel()
	src := "import ffi from std/ffi\n\nfn bad() -> *mut i32 {\n    let mut x: i32 = 5\n    ffi.withAddress(&mut x, (ptr: *mut i32) => ptr)\n}\n"
	out := checkStrict(t, src, false, 0)
	if !strings.Contains(out, "E1413") {
		t.Fatalf("expected E1413 carrier-lease-escape, got:\n%s", out)
	}
}

func TestCarrierLeaseLocalUseAccepted(t *testing.T) {
	t.Parallel()
	src := "import ffi from std/ffi\n\nfn ok() -> Bool {\n    let mut x: i32 = 5\n    ffi.withAddress(&mut x, (ptr: *mut i32) => ffi.isNull(ffi.toConst(ptr)))\n}\n"
	out := checkStrict(t, src, false, 0)
	if strings.Contains(out, "error") {
		t.Fatalf("expected the local-use lease to type-check, got:\n%s", out)
	}
}

// A bodiless std/ffi leaf with no StdOp id still counts as bounded under strict.
func TestStrictStackRawFFILeafAccepted(t *testing.T) {
	t.Parallel()
	src := "import ffi from std/ffi\n\nfn run() {\n    unsafe {\n        let p = ffi.toMut(ffi.null[i32]())\n        ffi.writeI32(p, 7)\n    }\n}\n\nrun()\nprint \"done\"\n"
	out := checkStrict(t, src, true, 0)
	if strings.Contains(out, "error") {
		t.Fatalf("expected a raw std/ffi leaf call to stay bounded under strict, got:\n%s", out)
	}
}

func TestCarrierLeaseEscapeViaRecordRejected(t *testing.T) {
	t.Parallel()
	src := "import ffi from std/ffi\n\ntype Box = {\n    pub p *mut i32\n}\n\nfn bad() -> Box {\n    let mut x: i32 = 5\n    ffi.withAddress(&mut x, (p: *mut i32) => Box { p: p })\n}\n"
	out := checkStrict(t, src, false, 0)
	if !strings.Contains(out, "E1413") {
		t.Fatalf("expected E1413 carrier-lease-escape through a record wrapper, got:\n%s", out)
	}
}

func TestCarrierLeaseRecordRejected(t *testing.T) {
	t.Parallel()
	src := "import ffi from std/ffi\n\ntype Counter = {\n    pub base Int\n}\n\nfn bad() -> Bool {\n    let mut c = Counter { base: 1 }\n    ffi.withAddress(&mut c, (p: *mut Counter) => ffi.isNull(ffi.toConst(p)))\n}\n"
	out := checkStrict(t, src, false, 0)
	if !strings.Contains(out, "E1414") {
		t.Fatalf("expected E1414 address-lease-type, got:\n%s", out)
	}
}

func TestStrictStackHostedDefaultUnchanged(t *testing.T) {
	t.Parallel()
	out := checkStrict(t, "fn fact(n: Int) -> Int {\n    if n <= 1 { 1 } else { n * fact(n - 1) }\n}\n\nlet total = fact(5)\nprint \"${total}\"\n", false, 0)
	if strings.Contains(out, "E1410") || strings.Contains(out, "E1411") || strings.Contains(out, "E1412") {
		t.Fatalf("expected the hosted-growable default to report only, got:\n%s", out)
	}
}

func analyzeModule(t *testing.T, main string, strict bool, budgetKiB int) *driver.Analysis {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"budget\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(main), 0o644)
	std, _ := filepath.Abs("../../../std")
	target := driver.HostTarget()
	target.StrictStack = strict
	target.StackBudgetKiB = budgetKiB
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// Recursion inside a withAddress callback is invisible to the MIR call graph otherwise.
func TestStrictStackCallbackRecursionRejected(t *testing.T) {
	t.Parallel()
	src := "import ffi from std/ffi\n\nfn recurse(n: Int) -> Int {\n    if n <= 0 { 0 } else { 1 + recurse(n - 1) }\n}\n\nfn run() -> Int {\n    let mut x: i32 = 0\n    ffi.withAddress(&mut x, (p: *mut i32) => recurse(1000000))\n}\n\nlet total = run()\nprint \"${total}\"\n"
	out := checkStrict(t, src, true, 0)
	if !strings.Contains(out, "E1410") || !strings.Contains(out, "unbounded") {
		t.Fatalf("expected E1410 stack-chain-unbounded through the withAddress callback, got:\n%s", out)
	}
}

// Same callback-recursion bypass as withAddress, via the broader hoCallbackArg class.
func TestStrictStackSharedWithCallbackRecursionRejected(t *testing.T) {
	t.Parallel()
	src := "import alloc from std/alloc\n\nfn recurse(n: Int) -> Int {\n    if n <= 0 { 0 } else { 1 + recurse(n - 1) }\n}\n\nfn run() -> Int {\n    let s = alloc.Shared.new(0)\n    s.with((v: &Int) => recurse(1000000))\n}\n\nlet total = run()\nprint \"${total}\"\n"
	out := checkStrict(t, src, true, 0)
	if !strings.Contains(out, "E1410") || !strings.Contains(out, "unbounded") {
		t.Fatalf("expected E1410 stack-chain-unbounded through the Shared.with callback, got:\n%s", out)
	}
}

func TestStrictStackCallbackBoundedAccepted(t *testing.T) {
	t.Parallel()
	src := "import ffi from std/ffi\n\nfn addOne(n: Int) -> Int {\n    n + 1\n}\n\nfn run() -> Int {\n    let mut x: i32 = 0\n    ffi.withAddress(&mut x, (p: *mut i32) => addOne(1))\n}\n\nlet total = run()\nprint \"${total}\"\n"
	out := checkStrict(t, src, true, 0)
	if strings.Contains(out, "error") {
		t.Fatalf("expected a bounded withAddress callback to pass strict, got:\n%s", out)
	}
}

func TestResourceReportStaticWritable(t *testing.T) {
	t.Parallel()
	withSecondMut := "extern(C) {\n    static foreignCounter: i32\n}\n\nstatic deviceId: u32 = 7\n\nstatic mut ticks: u32 = 0\n\nstatic mut ticks2: u32 = 0\n\nfn bump() -> u32 {\n    unsafe {\n        ticks = ticks + 1\n        ticks2 = ticks2 + 1\n        ticks + ticks2\n    }\n}\n\nlet total = bump()\nprint \"${total} ${deviceId}\"\n"
	withOneMut := "extern(C) {\n    static foreignCounter: i32\n}\n\nstatic deviceId: u32 = 7\n\nstatic mut ticks: u32 = 0\n\nfn bump() -> u32 {\n    unsafe {\n        ticks = ticks + 1\n        ticks\n    }\n}\n\nlet total = bump()\nprint \"${total} ${deviceId}\"\n"
	one := staticTotal(t, withOneMut)
	two := staticTotal(t, withSecondMut)
	if one == 0 || two != 2*one {
		t.Fatalf("expected a second static mut to double the total, got one=%d two=%d", one, two)
	}
}

func staticTotal(t *testing.T, src string) uint64 {
	t.Helper()
	report := driver.ResourceReport(analyzeModule(t, src, false, 0))
	for _, line := range report {
		if n, ok := strings.CutPrefix(line, "static: "); ok {
			n, _ = strings.CutSuffix(n, " bytes")
			var v uint64
			fmt.Sscan(n, &v)
			return v
		}
	}
	return 0
}

func TestResourceReportHostedDefault(t *testing.T) {
	t.Parallel()
	a := analyzeModule(t, "fn addOne(n: Int) -> Int {\n    n + 1\n}\n\nlet total = addOne(41)\nprint \"${total}\"\n", false, 0)
	report := driver.ResourceReport(a)
	if len(report) == 0 {
		t.Fatalf("expected a non-empty hosted resource report")
	}
	found := false
	for _, line := range report {
		if strings.HasPrefix(line, "main: bounded") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a bounded `main` entry, got: %v", report)
	}
}

func TestCarrierLeaseEscapeViaGenericWrapperRejected(t *testing.T) {
	t.Parallel()
	src := "import ffi from std/ffi\n\nfn lease[R](x: &mut i32, f: escaping fn(*mut i32) -> R) -> R {\n    ffi.withAddress(x, f)\n}\n\nfn bad() {\n    let mut v: i32 = 5\n    let p = lease(&mut v, (p: *mut i32) => p)\n    unsafe { ffi.writeI32(p, 123) }\n}\n"
	out := checkStrict(t, src, false, 0)
	if !strings.Contains(out, "E1413") {
		t.Fatalf("expected E1413 carrier-lease-escape through a generic wrapper, got:\n%s", out)
	}
}
