package driver_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

// Native hands the nullary variant's immortal over without allocating, so the VM counts nothing.
const nullaryBoxSrc = `type Tick = Up(n: Int) | Flat

fn same(v: Tick) -> Tick { v }

fn run(n: Int) -> Int {
    let pick: fn(Tick) -> Tick = same
    let t = if n > 0 { Up(n) } else { Flat }
    match pick(t) {
        Up(v) -> v
        Flat -> 0
    }
}

print "ok=${run(0)}"
`

func TestFaultInjectionNullaryBoxVM(t *testing.T) {
	t.Parallel()
	if code, out, errOut := vmAtAlloc(t, nullaryBoxSrc, 1+interpTicks); code != 0 || out != "ok=0\n" {
		t.Errorf("boxing a payload-free variant counted as an allocation: exit %d stdout=%q stderr=%q", code, out, errOut)
	}
}

// interpTicks is the print's own string; a program with no other allocation is refused at the tick after it.
const interpTicks = 1

func vmAtAlloc(t *testing.T, src string, at int64) (int, string, string) {
	t.Helper()
	a, err := driver.Analyze(loadTryConstructorModule(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	var out, errOut bytes.Buffer
	code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: at}, &out, &errOut)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	return code, out.String(), errOut.String()
}

// The generic returns no aggregate of T, so its scalar instantiation stays shared and the argument crosses.
const crossSrc = `fn pick%s { v is Some(_) }

fn run(n: Int) -> Int {
    if pick(Some(%s)) { n + 4 } else { n }
}

print "ok=${run(4)}"
`

func crossProgram(generic bool, payload string) string {
	sig := "(v: Int?) -> Bool"
	if generic {
		sig = "[T](v: T?) -> Bool"
	}
	return fmt.Sprintf(crossSrc, sig, payload)
}

// A fixed crossing reads the variant's immortal and the VM counts nothing; one built from a parameter is a box it counts.
func TestFaultInjectionFixedBoxVM(t *testing.T) {
	t.Parallel()
	if code, out, errOut := vmAtAlloc(t, crossProgram(true, "4"), 1+interpTicks); code != 0 || out != "ok=8\n" {
		t.Errorf("handing over a fixed variant counted as an allocation: exit %d stdout=%q stderr=%q", code, out, errOut)
	}
	if code, _, errOut := vmAtFirstAlloc(t, literalPrint(crossProgram(true, "n"))); code != 2 || !strings.Contains(errOut, "out of memory") {
		t.Errorf("boxing a variant built at run time was not refused: exit %d stderr=%q", code, errOut)
	}
}

// A program that prints a literal has no allocation of its own after the checked one, so N=1 lands on it.
func literalPrint(src string) string {
	return strings.Replace(src, "print \"ok=${run(4)}\"", "let k = run(4)\nprint \"ok\"", 1)
}
