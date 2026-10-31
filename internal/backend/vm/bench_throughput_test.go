package vm_test

import (
	"io"
	"sort"
	"syscall"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

var throughputCases = []struct{ name, src, want string }{
	{"Fib30", `
fn fib(n: Int) -> Int {
    if n < 2 {
        n
    } else {
        fib(n - 1) + fib(n - 2)
    }
}
print "${fib(30)}"
`, "832040\n"},
	{"IntLoop", `
let mut s = 0
let mut i = 0
for i < 1000000 {
    s = s + i % 7
    i = i + 1
}
print "${s}"
`, "2999997\n"},
	{"ArrayPushGetSum", `
let mut xs = Array.of(0)
for i in 0..100000 {
    xs.push(i)
}
let mut sum = 0
let mut k: usize = 0
for k < xs.len() {
    sum = sum + (xs.get(k) || 0)
    k = k + 1
}
print "${sum}"
`, "4999950000\n"},
	{"StringInterp", `
let mut n = 0
for i in 0..100000 {
    let s = "item ${i}: ${i * 2}"
    n = n + (s.len().toInt() || 0)
}
print "${n}"
`, "1733335\n"},
	{"MapInsertLookup", `
import map from std/map

let mut m = map.empty[Int, Int]()
for i in 0..20000 {
    m.set(i, i * 2)
}
let mut sum = 0
for i in 0..20000 {
    sum = sum + (m.get(&i) || 0)
}
print "${sum}"
`, "399980000\n"},
	{"ClosureCall", `
let add = (x: Int) => x + 1
let mut acc = 0
for i in 0..200000 {
    acc = add(acc)
}
print "${acc}"
`, "200000\n"},
	{"RecordFieldUpdate", `
type P = {
    pub mut x Int
    pub mut y Int
}
let mut p = P { x: 0, y: 0 }
for i in 0..200000 {
    p.x = p.x + 1
    p.y += p.x
}
print "${p.x} ${p.y}"
`, "200000 20000100000\n"},
	{"MatchADT", `
type Shape = Circle(r: Int) | Rect(w: Int, h: Int) | Dot

fn area(s: &Shape) -> Int {
    match s {
        Circle(r) -> 3 * r * r
        Rect { w, h } -> w * h
        Dot -> 1
    }
}

let mut shapes = Array.of(Dot)
for i in 0..30000 {
    if i % 3 == 0 {
        shapes.push(Circle(i % 10))
    } else if i % 3 == 1 {
        shapes.push(Rect(i % 7, 2))
    } else {
        shapes.push(Dot)
    }
}
let mut total = 0
for round in 0..5 {
    for s in &shapes {
        total = total + area(s)
    }
}
print "${total}"
`, "4624965\n"},
}

func benchProgram(tb testing.TB, src string) *mir.Program {
	tb.Helper()
	pkgs := map[string]*sem.Package{}
	for _, p := range testkit.LoadStd(tb, "../../../std") {
		pkgs[p.Path] = p
	}
	tree := syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	if tree.HasErrors() {
		tb.Fatalf("parse errors: %v", tree.Diags)
	}
	pkgs["main"] = &sem.Package{Path: "main", Files: []*syntax.Tree{tree}, Entry: tree}
	var paths []string
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	mod := &sem.Module{}
	for _, p := range paths {
		mod.Packages = append(mod.Packages, pkgs[p])
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		tb.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	plans := []mir.Transform{{Name: "plan-rc", Run: mir.PlanRC}, {Name: "specialize", Run: mir.Specialize}, {Name: "elide-rc", Run: mir.ElideRC}, {Name: "plan-direct", Run: mir.PlanDirect}}
	if err := prog.Apply(plans...); err != nil {
		tb.Fatalf("plans: %v", err)
	}
	return prog
}

func TestThroughputProgramsPrintTheirExpectedOutput(t *testing.T) {
	for _, c := range throughputCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var out bytesSink
			if code, err := vm.Run(benchProgram(t, c.src), vm.Options{}, &out, io.Discard); err != nil || code != 0 {
				t.Fatalf("run: code %d, err %v", code, err)
			}
			if string(out) != c.want {
				t.Errorf("output %q, want %q", out, c.want)
			}
		})
	}
}

// BenchmarkThroughput times the interpreter loop on one program per hot shape.
func BenchmarkThroughput(b *testing.B) {
	for _, c := range throughputCases {
		b.Run(c.name, func(b *testing.B) {
			prog := benchProgram(b, c.src)
			b.ReportAllocs()
			before := cpuTime()
			for b.Loop() {
				var out bytesSink
				if code, err := vm.Run(prog, vm.Options{}, &out, io.Discard); err != nil || code != 0 {
					b.Fatalf("run: code %d, err %v", code, err)
				}
				if c.want != "" && string(out) != c.want {
					b.Fatalf("output %q, want %q", out, c.want)
				}
			}
			b.ReportMetric(float64(cpuTime()-before)/float64(b.N), "cpu-ns/op")
		})
	}
}

type bytesSink []byte

func (s *bytesSink) Write(p []byte) (int, error) { *s = append(*s, p...); return len(p), nil }

// cpuTime is the process's user plus system CPU in nanoseconds, which a loaded machine does not stretch the way wall time is.
func cpuTime() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return ru.Utime.Nano() + ru.Stime.Nano()
}
