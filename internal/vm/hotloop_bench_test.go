package vm_test

import (
	"io"
	"sort"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
	"kigumi/internal/vm"
)

// hotLoopSrc pushes onto an Array repeatedly: std/array.push has a Go accelerator, toggled off by NoAccel.
const hotLoopSrc = `
let mut total = 0
for i in 0..2000 {
    let mut xs = Array.of(0)
    for j in 0..50 {
        xs.push(j)
    }
    total = total + (xs.len().toInt() || 0)
}
print "${total}"
`

// buildRunnableProgramTB is buildRunnableProgram over testing.TB, so a *testing.B can share it.
func buildRunnableProgramTB(tb testing.TB, src string) *mir.Program {
	tb.Helper()
	pkgs := map[string]*sem.Package{}
	for _, p := range testkit.LoadStd(tb, "../../std") {
		pkgs[p.Path] = p
	}
	tree := syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	if tree.HasErrors() {
		for _, d := range tree.Diags {
			tb.Logf("%s", d.Msg)
		}
		tb.Fatalf("parse errors: %d diagnostics", len(tree.Diags))
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
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		tb.Fatalf("plan-rc: %v", err)
	}
	return prog
}

// BenchmarkVMHotLoopAccelerated and BenchmarkVMHotLoopReference run the same hot loop with std/array.push's accelerator on and off.
func BenchmarkVMHotLoopAccelerated(b *testing.B) { benchmarkHotLoop(b, false) }
func BenchmarkVMHotLoopReference(b *testing.B)   { benchmarkHotLoop(b, true) }

func benchmarkHotLoop(b *testing.B, noAccel bool) {
	prog := buildRunnableProgramTB(b, hotLoopSrc)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := vm.Run(prog, vm.Options{NoAccel: noAccel}, io.Discard, io.Discard); err != nil {
			b.Fatalf("run: %v", err)
		}
	}
}

// textBenchBodies are text and bytes operations whose std bodies are Kigumi; each runs over the same line in a loop.
var textBenchBodies = []struct{ name, body string }{
	{"trim", `let t = line.trim()`},
	{"split", `let t = line.split(" ")`},
	{"lines", `let t = text.lines(para)`},
	{"chars", `let t = line.chars()`},
	{"graphemes", `let t = text.graphemes(line)`},
	{"toUpper", `let t = line.toUpper()`},
	{"replace", `let t = text.replace(line, "fox", "cat")`},
	{"join", `let t = array.join(&words, ", ")`},
	{"trimPrefix", `let t = text.trimPrefix(line, "  the")`},
	{"contains", `let t = line.contains("lazy dog")`},
	{"indexOf", `let t = line.toBytes().indexOf("lazy".toBytes())`},
	{"lastIndexOf", `let t = line.toBytes().lastIndexOf("the".toBytes())`},
	{"concat", `let t = line.concat(line)`},
	{"bytesConcat", `let t = line.toBytes().concat(line.toBytes())`},
	{"fill", `let t = Bytes.fill(65, 64)`},
	{"sortedBy", `let t = array.sortedBy(&words, (a, b) => a.compareTo(b))`},
	{"builder", "let mut sb = text.StringBuilder.new()\n    sb.push(line)\n    sb.pushByte(33)\n    let t = sb.freeze()"},
}

func textBenchSrc(body string) string {
	return `import array from std/array
import text from std/text

let line = "  the quick brown fox jumps over the lazy dog, the end  "
let para = "alpha beta\ngamma delta\r\nepsilon\n"
let words = line.split(" ")
for i in 0..300 {
    ` + body + `
}
print "done"
`
}

// BenchmarkVMText runs each text operation with the host accelerators on and with NoAccel, where the Kigumi body runs.
func BenchmarkVMText(b *testing.B) {
	for _, c := range textBenchBodies {
		prog := buildRunnableProgramTB(b, textBenchSrc(c.body))
		for _, mode := range []struct {
			name    string
			noAccel bool
		}{{"accel", false}, {"body", true}} {
			b.Run(c.name+"/"+mode.name, func(b *testing.B) {
				for b.Loop() {
					if _, err := vm.Run(prog, vm.Options{NoAccel: mode.noAccel}, io.Discard, io.Discard); err != nil {
						b.Fatalf("run: %v", err)
					}
				}
			})
		}
	}
}

// arrayBenchBodies are array operations whose std bodies are Kigumi; each runs over the same 64 elements in a loop.
var arrayBenchBodies = []struct{ name, body string }{
	{"clone", `let t = nums.clone()`},
	{"take", `let t = nums.take(32)`},
	{"reverse", `let t = nums.reverse()`},
	{"concat", `let t = nums.concat(Array.of(1, 2, 3))`},
	{"appendAll", "let mut t = Array.of(0)\n    t.appendAll(Array.of(1, 2, 3, 4, 5, 6, 7, 8))"},
	{"cloneStrings", `let t = words.clone()`},
	{"mapKeys", `let t = m.keys()`},
}

func arrayBenchSrc(body string) string {
	return `import map from std/map

let nums = Array.tryFill(7, 64).unwrap()
let words = "a b c d e f g h i j k l m n o p q r s t u v w x y z".split(" ")
let mut m = map.empty[Int, Int]()
for k in 0..32 {
    m.set(k, k)
}
for i in 0..300 {
    ` + body + `
}
print "done"
`
}

// BenchmarkVMArray runs each array operation with the host accelerators on and with NoAccel, where the Kigumi body runs.
func BenchmarkVMArray(b *testing.B) {
	for _, c := range arrayBenchBodies {
		prog := buildRunnableProgramTB(b, arrayBenchSrc(c.body))
		for _, mode := range []struct {
			name    string
			noAccel bool
		}{{"accel", false}, {"body", true}} {
			b.Run(c.name+"/"+mode.name, func(b *testing.B) {
				for b.Loop() {
					if _, err := vm.Run(prog, vm.Options{NoAccel: mode.noAccel}, io.Discard, io.Discard); err != nil {
						b.Fatalf("run: %v", err)
					}
				}
			})
		}
	}
}
