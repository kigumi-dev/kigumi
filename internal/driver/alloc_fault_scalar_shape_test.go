package driver_test

import (
	"fmt"
	"testing"
)

type scalarShapeCase struct {
	name, src string
	perRound  int
}

// Each source takes the round count for %[1]d.
var scalarShapeCases = []scalarShapeCase{
	{"helper forwarding to a fn value", `fn big(x: Int) -> Int { x * 1000 + 7 }
fn viaFn(f: fn(Int) -> Int, x: Int) -> Int { f(x) }
let f: fn(Int) -> Int = big
let mut s = 0
for i in 0..%[1]d { s = s + viaFn(f, i + 2000) }
print "${s > 0}"`, 2},
	{"callback result from a std map", `let xs = Array.fill(2000, %[1]d).map((x: &Int) => x * 3 + 1)
print "${xs.len()}"`, 1},
	{"closure writing a captured local", `let mut c = 0
let add = (d: Int) => { c = c + d }
for i in 0..%[1]d { add(i + 2000) }
print "${c > 0}"`, 2},
	{"known closure over a large int", `let k = 5
let f = (x: Int) => x + k
let mut s = 0
for i in 0..%[1]d { s = s + f(i + 2000) }
print "${s > 0}"`, 2},
	{"hash through a std witness", `import map from std/map
let mut m = map.empty[Int, Int]()
m.set(1, 1)
let mut s = 0
for i in 0..%[1]d { if m.contains(&1) { s = s + 1 } }
print "${s}"`, 0},
	{"loop over a large array", `let xs = Array.fill(7, 1500)
let mut s = 0
for r in 0..%[1]d { for x in &xs { s = s + x } }
print "${s > 0}"`, 0},
	{"store through a mut reference", `fn bump(r: &mut Int, i: Int) { r = r + i + 2000 }
let mut c = 0
for i in 0..%[1]d { bump(&mut c, i) }
print "${c > 0}"`, 1},
	{"store into a record through a reference", `type P = {
    pub mut a Int
    pub mut b Float
}
fn bump(p: &mut P, i: Int) {
    p.a = i + 2000
    p.b = 0.5
}
let mut p = P { a: 0, b: 0.0 }
for i in 0..%[1]d { bump(&mut p, i) }
print "${p.a > 0}"`, 1},
	{"store into an array element", `fn put(xs: &mut Array[Int], i: Int) { xs[0] = i + 2000 }
let mut xs = Array.fill(0, 2)
for i in 0..%[1]d { put(&mut xs, i) }
print "${xs[0] > 0}"`, 1},
	{"identity closure on a large u64", `let f = (x: u64) => x
let mut s: u64 = 0
for i in 0..%[1]d { s = s + f(s + 5000) }
print "${s > 0}"`, 1},
	{"wide int through a fn value", `fn big(x: i128) -> i128 { x * 3 }
let f: fn(i128) -> i128 = big
let mut s: i128 = 0
for i in 0..%[1]d { s = s + f(7) }
print "${s > 0}"`, 1},
	{"non-ascii char from a std map", `let cs = Array.fill(1, %[1]d).map((x: &Int) => Char.fromInt(x + 20000) || 'a')
print "${cs.len()}"`, 1},
}

func TestFaultInjectionScalarShapesCountTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range scalarShapeCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			short, long := fmt.Sprintf(c.src, 4), fmt.Sprintf(c.src, 12)
			vmPer := vmAllocs(t, long) - vmAllocs(t, short)
			nativePer := nativeAllocs(t, long) - nativeAllocs(t, short)
			if vmPer != nativePer || nativePer != 8*c.perRound {
				t.Errorf("8 more rounds cost %d allocations on the VM and %d on native, want %d on both", vmPer, nativePer, 8*c.perRound)
			}
		})
	}
}
