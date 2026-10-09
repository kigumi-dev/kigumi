package driver_test

import (
	"bytes"
	"fmt"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

const boxSiteRounds = 4

type boxSiteCase struct {
	name string
	// decls declare the crossed type and the callee; arg builds one value per round.
	decls, ty, callee, arg string
}

const boxSiteTemplate = `%s
fn run(f: fn(%s) -> Bool, n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        if f(%s) { s = s + 1 }
    }
    s
}

print "ok=${run(%s, %d)}"
`

const boxSiteBaseline = `fn flag(b: Bool) -> Bool { b }
` + boxSiteTemplate

// Bool is immortal on native, so the baseline's calls allocate nothing and the difference is the box.
var boxSiteCases = []boxSiteCase{
	{
		name:  "record of bools",
		decls: "type Flags = {\n    pub a Bool\n    pub b Bool\n}\n\nfn use(v: Flags) -> Bool { v.a && v.b }\n",
		ty:    "Flags", callee: "use", arg: "Flags { a: i > 0, b: true }",
	},
	{
		name:  "record of ints",
		decls: "type Point = {\n    pub x Int\n    pub y Int\n}\n\nfn use(v: Point) -> Bool { v.x < v.y }\n",
		ty:    "Point", callee: "use", arg: "Point { x: i, y: 1 }",
	},
	{
		name:  "payload variant",
		decls: "type Mark = On(n: Int) | Off\n\nfn use(m: Mark) -> Bool {\n    match m {\n        On(n) -> n > 0\n        Off -> false\n    }\n}\n",
		ty:    "Mark", callee: "use", arg: "On(i)",
	},
	{
		name:  "nested aggregate",
		decls: "type Inner = {\n    pub a Int\n    pub b Float\n}\n\ntype Outer = {\n    pub inner Inner\n    pub c Char\n    pub ok Bool\n}\n\nfn use(o: Outer) -> Bool { o.ok && o.inner.a > 0 }\n",
		ty:    "Outer", callee: "use", arg: "Outer { inner: Inner { a: i, b: 1.5 }, c: 'x', ok: true }",
	},
	{
		name:  "tuple",
		decls: "fn use(t: (Int, Bool)) -> Bool { t._1 }\n",
		ty:    "(Int, Bool)", callee: "use", arg: "(i, true)",
	},
	{
		name:  "fixed array",
		decls: "fn use(a: FixedArray[Int, 5]) -> Bool { (a.get(0) || 0) > 0 }\n",
		ty:    "FixedArray[Int, 5]", callee: "use", arg: "FixedArray.filled[Int, 5](i)",
	},
}

func (c boxSiteCase) crossing(rounds int) string {
	return fmt.Sprintf(boxSiteTemplate, c.decls, c.ty, c.arg, c.callee, rounds)
}

func (c boxSiteCase) baseline(rounds int) string {
	return fmt.Sprintf(boxSiteBaseline, "", "Bool", "i > 0", "flag", rounds)
}

func vmAllocs(t *testing.T, src string) int {
	t.Helper()
	a, err := driver.Analyze(loadTryConstructorModule(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	for n := 1; n <= directBoxSweep*4; n++ {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n)}, &out, &errOut)
		if err != nil {
			t.Fatalf("N=%d: run error: %v", n, err)
		}
		if !checkBoxOutcome(t, "vm", n, code, out.String(), errOut.String()) {
			return n - 1
		}
	}
	t.Fatalf("vm still refuses at N=%d", directBoxSweep*4)
	return 0
}

func TestFaultInjectionBoxSitesCountTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range boxSiteCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			vmBoxes := vmAllocs(t, c.crossing(boxSiteRounds)) - vmAllocs(t, c.baseline(boxSiteRounds))
			nativeBoxes := nativeAllocs(t, c.crossing(boxSiteRounds)) - nativeAllocs(t, c.baseline(boxSiteRounds))
			if vmBoxes != nativeBoxes || vmBoxes < boxSiteRounds {
				t.Errorf("box sites cost %d allocations on the VM and %d on native, want the same and at least %d", vmBoxes, nativeBoxes, boxSiteRounds)
			}
		})
	}
}

const erasedClosureSrc = `fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        let a = i
        let b = 2.5
        let f: fn() -> Bool = () => a > 0 && b > 1.0
        if f() { s = s + 1 }
    }
    s
}

print "ok=${run(%d)}"
`

const keptClosureSrc = `fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        let a = i
        let b = 2.5
        let f = () => a > 0 && b > 1.0
        if f() { s = s + 1 }
    }
    s
}

print "ok=${run(%d)}"
`

func TestFaultInjectionErasedClosureCountsTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	erased, kept := fmt.Sprintf(erasedClosureSrc, boxSiteRounds), fmt.Sprintf(keptClosureSrc, boxSiteRounds)
	vmBoxes := vmAllocs(t, erased) - vmAllocs(t, kept)
	nativeBoxes := nativeAllocs(t, erased) - nativeAllocs(t, kept)
	if vmBoxes != nativeBoxes || vmBoxes < boxSiteRounds {
		t.Errorf("erasing a closure costs %d allocations on the VM and %d on native, want the same and at least %d", vmBoxes, nativeBoxes, boxSiteRounds)
	}
}
