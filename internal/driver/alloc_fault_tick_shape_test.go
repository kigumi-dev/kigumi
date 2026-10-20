package driver_test

import (
	"fmt"
	"testing"

	"kigumi/internal/driver"
)

// Each source takes the round count for %[1]d.
var tickShapeCases = []scalarShapeCase{
	{"closure returned from a function", `fn mk(k: Int) -> fn(Int) -> Int { (x: Int) => x + k }
let f = mk(5)
let mut s = 0
for i in 0..%[1]d { s = s + f(i + 3000) }
print "${s > 0}"`, 2},
	{"fn item returned from a function", `fn twice(x: Int) -> Int { x * 2 }
fn pick() -> fn(Int) -> Int { twice }
let f = pick()
let mut s = 0
for i in 0..%[1]d { s = s + f(i + 2000) }
print "${s > 0}"`, 2},
	{"closure forwarding its parameter to another closure", `let k = 5
let f: fn(Int) -> Int = (x: Int) => x + k
let g: fn(Int) -> Bool = (y: Int) => f(y) > 0
let mut s = 0
for i in 0..%[1]d { if g(i + 3000) { s = s + 1 } }
print "${s}"`, 2},
	{"float fn item with a constant argument", `fn half(x: Float) -> Float { x / 2.0 }
fn ap(f: fn(Float) -> Float, x: Float) -> Float { f(x) }
let mut s = 0.0
for i in 0..%[1]d { s = s + ap(half, 1.5) }
print "${s > 0.0}"`, 1},
	{"float fn item with a computed argument", `fn half(x: Float) -> Float { x / 2.0 }
fn ap(f: fn(Float) -> Float, x: Float) -> Float { f(x) }
fn run(n: Int) -> Float {
    let mut s = 0.0
    for i in 0..n { s = s + ap(half, i.toFloat()) }
    s
}
print "${run(%[1]d) > 0.0}"`, 2},
	{"int fn item with a constant argument", `fn twice(x: Int) -> Int { x * 2 }
fn ap(f: fn(Int) -> Int, x: Int) -> Int { f(x) }
let mut s = 0
for i in 0..%[1]d { s = s + ap(twice, 3000) }
print "${s > 0}"`, 1},
	{"int fn item with a computed argument", `fn twice(x: Int) -> Int { x * 2 }
fn ap(f: fn(Int) -> Int, x: Int) -> Int { f(x) }
let mut s = 0
for i in 0..%[1]d { s = s + ap(twice, i + 3000) }
print "${s > 0}"`, 2},
	{"captured local handed to a fn value", `fn run(n: Int) -> Int {
    let h: fn(Int) -> Bool = (y: Int) => y > 0
    let k = n + 3000
    let f: fn(Int) -> Bool = (x: Int) => h(k)
    let mut s = 0
    for i in 0..n { if f(i) { s = s + 1 } }
    s
}
print "${run(%[1]d)}"`, 0},
	{"toFloat inside a map callback", `let ys = Array.fill(2000, %[1]d).map((x: &Int) => x.toFloat() * 1.5)
print "${ys.len()}"`, 1},
	{"toFloat handed to a fn value", `fn run(n: Int) -> Int {
    let f: fn(Float) -> Bool = (x: Float) => x > 1.0
    let mut s = 0
    for i in 0..n { if f(i.toFloat() * 1.5) { s = s + 1 } }
    s
}
print "${run(%[1]d)}"`, 1},
	{"parameter stored through a reference, computed argument", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
let mut c = 0
for i in 0..%[1]d { put(&mut c, i + 2000) }
print "${c > 0}"`, 1},
	{"parameter stored through a reference, constant argument", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
let mut c = 0
for i in 0..%[1]d { put(&mut c, 3000) }
print "${c > 0}"`, 0},
	{"parameter forwarded to a storing function", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
fn put2(r: &mut Int, i: Int) -> Unit { put(r, i) }
let mut c = 0
for i in 0..%[1]d { put2(&mut c, i + 2000) }
print "${c > 0}"`, 1},
	{"parameter stored into an array element, constant argument", `fn put(xs: &mut Array[Int], i: Int) -> Unit { xs[0] = i }
let mut xs = Array.fill(0, 2)
for i in 0..%[1]d { put(&mut xs, 3000) }
print "${xs[0] > 0}"`, 0},
	{"parameter stored, argument is a user call result", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
fn f(x: Int) -> Int { x * 3 }
let mut c = 0
for i in 0..%[1]d { put(&mut c, f(i + 1000)) }
print "${c > 0}"`, 1},
	{"parameter stored, argument is a mutable accumulator", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
let mut c = 0
let mut k = 2000
for i in 0..%[1]d { k = k + 1
    put(&mut c, k) }
print "${c > 0}"`, 1},
	{"parameter stored, argument is a range loop variable", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
let mut c = 0
for i in 1500..(1500 + %[1]d) { put(&mut c, i) }
print "${c > 0}"`, 1},
	{"parameter stored, argument is a field of a direct record", `type P = { x Int, y Int }
fn put(r: &mut Int, i: Int) -> Unit { r = i }
let mut c = 0
for i in 0..%[1]d { let p = P { x: i + 2000, y: 1 }
    put(&mut c, p.x) }
print "${c > 0}"`, 1},
	{"mutual recursion reached through the storing function", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
fn f(r: &mut Int, i: Int, n: Int) -> Unit {
    if n > 0 { g(r, i, n - 1) }
    put(r, i)
}
fn g(r: &mut Int, i: Int, n: Int) -> Unit {
    if n > 0 { f(r, i, n - 1) }
}
let mut c = 0
for i in 0..%[1]d { f(&mut c, i + 2000, 1) }
print "${c > 0}"`, 1},
	{"mutual recursion, callee reached first through the storing function", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
fn f(r: &mut Int, i: Int, n: Int) -> Unit {
    if n > 0 { g(r, i, n - 1) }
    put(r, i)
}
fn g(r: &mut Int, i: Int, n: Int) -> Unit {
    if n > 0 { f(r, i, n - 1) }
}
let mut c = 0
f(&mut c, 7, 0)
for i in 0..%[1]d { g(&mut c, i + 2000, 0) }
print "${c > 0}"`, 1},
	{"mutual recursion, callee reached directly", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
fn f(r: &mut Int, i: Int, n: Int) -> Unit {
    if n > 0 { g(r, i, n - 1) }
    put(r, i)
}
fn g(r: &mut Int, i: Int, n: Int) -> Unit {
    if n > 0 { f(r, i, n - 1) }
}
let mut c = 0
for i in 0..%[1]d { g(&mut c, i + 2000, 0) }
print "${c > 0}"`, 1},
	{"parameter stored, argument is an if value", `fn put(r: &mut Int, i: Int) -> Unit { r = i }
let mut c = 0
for i in 0..%[1]d { let v = if i > 100 { i + 2000 } else { i + 3000 }
    put(&mut c, v) }
print "${c > 0}"`, 1},
}

func TestFaultInjectionTickShapesCountTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range tickShapeCases {
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

type tickPairCase struct {
	name, computed, constant string
	perRound                 int
}

const tickPairTemplate = `fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        %s
    }
    s
}
print "${run(%d)}"
`

// The scalar boxes of a shape cost the difference to the same shape over constants.
var tickPairCases = []tickPairCase{
	{"Option.map with a computed int", "let p = Some(i + 5000).map((x: Int) => x * 2)\n        if p is Some(_) { s = s + 1 }", "let p = Some(i + 5000).map((x: Int) => 7)\n        if p is Some(_) { s = s + 1 }", 1},
	{"Option.map to a float", "let p = Some(i + 5000).map((x: Int) => x.toFloat())\n        if p is Some(_) { s = s + 1 }", "let p = Some(i + 5000).map((x: Int) => 2.5)\n        if p is Some(_) { s = s + 1 }", 1},
	{"Option.map over a float", "let p = Some(2.5).map((x: Float) => x * 2.0)\n        if p is Some(_) { s = s + 1 }", "let p = Some(2.5).map((x: Float) => 3.5)\n        if p is Some(_) { s = s + 1 }", 1},
	{"parameter stored through a local reference", "let mut c = 0\n        put(&mut c, i + 2000)\n        s = s + c", "let mut c = 0\n        put(&mut c, 3000)\n        s = s + c", 1},
}

func (c tickPairCase) source(body string, rounds int) string {
	return "fn put(r: &mut Int, i: Int) -> Unit { r = i }\n" + fmt.Sprintf(tickPairTemplate, body, rounds)
}

func TestFaultInjectionTickPairsCountTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range tickPairCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cost := func(count func(*testing.T, string) int, body string) int {
				return count(t, c.source(body, 12)) - count(t, c.source(body, 4))
			}
			vmPer := cost(vmAllocs, c.computed) - cost(vmAllocs, c.constant)
			nativePer := cost(nativeAllocs, c.computed) - cost(nativeAllocs, c.constant)
			if vmPer != nativePer || nativePer != 8*c.perRound {
				t.Errorf("computing costs %d more allocations on the VM and %d on native over 8 rounds, want %d on both", vmPer, nativePer, 8*c.perRound)
			}
		})
	}
}

var tickAggregateCases = []tickPairCase{
	{name: "Some built inside Option.map", computed: "let p = Some(i + 5000).map((x: Int) => x * 2)\n        if p is Some(_) { s = s + 1 }"},
	{name: "array literal of computed ints", computed: "let xs = Array.of(i + 5000, 2)\n        if xs.len() > 1 { s = s + 1 }"},
	{name: "empty array literal", computed: "let xs: Array[Int] = Array.of()\n        if xs.len() < 1 { s = s + 1 }"},
}

func TestFaultInjectionBuiltAggregatesCountTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range tickAggregateCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			vmPer := vmAllocs(t, c.source(c.computed, 12)) - vmAllocs(t, c.source(c.computed, 4))
			nativePer := nativeAllocs(t, c.source(c.computed, 12)) - nativeAllocs(t, c.source(c.computed, 4))
			if vmPer != nativePer || nativePer == 0 {
				t.Errorf("8 more rounds cost %d allocations on the VM and %d on native, want the same and not zero", vmPer, nativePer)
			}
		})
	}
}
