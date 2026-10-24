package driver_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/llgen"
	"kigumi/internal/sem"
)

const noallocIRPrelude = `import {Box} from std/alloc

type Nm = {
    pub name String
    pub mut n Int
}

type W = {
    pub mut nm Nm
    pub mut k Int
}

type Pt = {
    pub mut x Int
    pub mut y Int
}

type Tag = A | B(v: Int) | C(nm: Nm)

type Tree = Leaf(v: Int) | Node(kids: FixedArray[Tree, 2])

fn mk() -> Nm { Nm { name: "a", n: 1 } }
noalloc fn take(n: Nm) -> Int { n.n }
noalloc fn takeW(w: W) -> Int { w.k }
noalloc fn takeT(t: Tag) -> Int { match t { A -> 0, B(v) -> v, C(c) -> c.n } }
noalloc fn takeP(p: Pt) -> Int { p.x }

noalloc fn applyGen[F: Fn[noalloc fn(Int) -> Bool]](f: F) -> Bool {
    f(1)
}

noalloc fn applyDyn(f: noalloc fn(Int) -> Bool) -> Bool {
    f(1)
}

noalloc fn putI(r: &mut Int, i: Int) { r = i }
noalloc fn putF(p: &mut Pt, f: Float) { p.x = 1 }
noalloc fn callI(f: noalloc fn(Int) -> Bool, x: Int) -> Bool { f(x) }
noalloc fn fwdI(r: &mut Int, i: Int) { putI(r, i) }

`

var allocatingCall = regexp.MustCompile(`call [^@\n]*@(rt_copy|rt_copy_mut|rt_record|rt_variant|rt_array|rt_closure|rt_future_new|malloc|calloc|rt_float|rt_box_int128)\(|call [^@\n]*@(rt_int|rt_char)\([a-z0-9]+ %`)

// noallocBodies compiles src and returns the allocating runtime calls found in the body of every function declared noalloc and every closure.
func noallocBodies(t *testing.T, src string) (rejected string, checked int, holes []string) {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"probe\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		return "analyze: " + err.Error(), 0, nil
	}
	if a.Res.HasErrors() {
		return a.Res.Render(), 0, nil
	}
	audited := map[string]bool{}
	for _, f := range a.Prog.Funcs {
		if f.Ent == 0 || !strings.HasPrefix(f.Name, "probe/main.") {
			continue
		}
		switch e := a.Res.Entity(f.Ent); e.Kind {
		case sem.EntClosure:
			audited[f.Name] = true
		case sem.EntFn:
			audited[f.Name] = a.Res.Fn(f.Ent).Declared&sem.EffNoalloc != 0
		}
	}
	ir := llgen.EmitProgram(a.Prog, llgen.EmitOptions{}).IR
	for _, chunk := range strings.Split(ir, "\ndefine ")[1:] {
		head, _, _ := strings.Cut(chunk, "\n}\n")
		name := strings.Trim(head[strings.Index(head, "@")+1:strings.Index(head, "(")], `"`)
		if !audited[name] {
			continue
		}
		checked++
		for _, mm := range allocatingCall.FindAllStringSubmatch(head, -1) {
			holes = append(holes, name+" -> "+mm[1]+mm[2])
		}
	}
	return "", checked, holes
}

type noallocProbe struct {
	src    string
	reject string
}

var noallocProbes = []noallocProbe{
	{"noalloc fn p0(n: Nm) -> Int { let mut m = n\n m.n = 4\n n.n }", "E1700"},
	{"noalloc fn p1(a: &Array[Nm]) -> Int { if a.get(0) is Some(v) { v.n } else { 0 } }", "E1700"},
	{"noalloc fn p2(xs: FixedArray[Nm, 2]) -> Int { let mut t = 0\n for x in xs { t = t + x.n }\n t }", ""},
	{"noalloc fn p3(w: &mut W, q: Nm) -> Int { w.nm = q\n q.n }", "E1700"},
	{"noalloc fn p4(n: Nm) -> Int { take(n) + take(n) }", ""},
	{"noalloc fn p5(n: Nm) -> Int { let w = W { nm: n, k: 1 }\n w.k + n.n }", "E961"},
	{"noalloc fn p6(n: Nm, xs: &mut FixedArray[Nm, 2]) -> Int { xs[0] = n\n xs[1] = n\n 0 }", "E1700"},
	{"noalloc fn p7(xs: &FixedArray[Nm, 2]) -> Int { let t = xs[0]\n t.n }", ""},
	{"noalloc fn p8(w: &W) -> Int { let a = w.nm\n let b = w.nm\n a.n + b.n }", ""},
	{"noalloc fn p9(w: &W) -> Int { take(w.nm) }", ""},
	{"noalloc fn p10(w: W) -> Int { takeW(w) + takeW(w) }", ""},
	{"noalloc fn p11(t: Tag) -> Int { takeT(t) + takeT(t) }", ""},
	{"noalloc fn p12(t: &Tag) -> Int { match t { A -> 0, B(v) -> v, C(c) -> take(c) } }", "E1700"},
	{"noalloc fn p13(o: Option[Nm]) -> Int { match o { Some(v) -> take(v), None -> 0 } }", ""},
	{"noalloc fn p14(w: &mut W, v: Nm) { let tmp = w.nm\n w.nm = v\n w.nm = tmp }", "E1700"},
	{"noalloc fn p15(a: &mut Array[Nm]) -> Int { a.withMut(0, (x) => { x.n = 3 })\n 0 }", "E907"},
	{"noalloc fn p16(n: Nm) -> Int { let f = () => n.n\n f() }", "E961"},
	{"noalloc fn p17(p: Pt) -> Int { let q = p\n takeP(q) + takeP(p) }", ""},
	{"noalloc fn p18(p: &Pt) -> Int { let q = p\n q.x }", ""},
	{"noalloc fn p19(xs: &Array[Nm]) -> Int { let mut t = 0\n for x in xs { t = t + x.n }\n t }", ""},
	{"noalloc fn p20(t: Tree) -> Int { match t { Leaf(v) -> v, Node(kids) -> { let mut k = kids\n k[0] = Leaf(1)\n 0 } } }", "E1700"},
	{"noalloc fn p21(w: &W) -> Int { let mut n = w.nm\n n.n = 4\n w.nm.n }", "E1700"},
	{"noalloc fn p22(n: Nm) -> Nm { let m = n\n m }", "E1700"},
	{"noalloc fn p23(w: &mut W) -> Int { let a = w.nm\n w.nm.n = a.n + 1\n w.nm.n }", "E2125"},
	{"noalloc fn p24(xs: &Array[Nm]) -> Int { if xs.first() is Some(v) { v.n } else { 0 } }", "E1700"},
	{"noalloc fn p25(n: Nm) -> Int { let mut t = 0\n let mut i = 0\n for i < 3 { t = t + take(n)\n i = i + 1 }\n t }", ""},
	{"noalloc fn p26(a: Nm) -> Int { let r = if a.n > 0 { a } else { mk() }\n r.n }", "E907"},
	{"noalloc fn s0(xs: &mut FixedArray[Pt, 2]) { let t = xs[0]\n xs[0] = xs[1]\n xs[1] = t }", "E1610"},
	{"noalloc fn s1(b: &Bytes, sub: Bytes) -> Bool { b.startsWith(sub) }", "E1700"},
	{"noalloc fn s2(b: &Bytes, sub: Bytes) -> Bool { b.endsWith(sub) }", "E1700"},
	{"noalloc fn s3(b: Bytes, c: Bytes) -> usize { b.len() + c.len() }", ""},
	{"noalloc fn s4(p: &mut Pt, q: Pt) { let t = p.x\n p.x = q.x\n p.y = t }", "E2125"},
	{"noalloc fn s5(xs: &mut Array[Pt]) { if xs.get(0) is Some(v) { xs.withMut(0, (e) => { e.x = v.y }) } }", "E907"},
	{"noalloc fn s6(w: &mut W, q: Pt) -> Int { let a = q\n let b = a\n a.x + b.y }", ""},
	{"noalloc fn s7(t: Tree) -> Int { match t { Leaf(v) -> v, Node(kids) -> { let a = kids[0]\n 0 } } }", ""},
	{"noalloc fn b0(b: &Box[Int]) -> Int { b.get() }", ""},
	{"noalloc fn b1(b: &Box[Pt]) -> Int { b.get().x }", "E1700"},
	{"noalloc fn b2(b: &Box[Nm]) -> Int { b.with((n: &Nm) => n.n) }", ""},
	{"noalloc fn b3(b: &mut Box[Pt]) -> Int { b.withMut((p: &mut Pt) => { p.x = p.x + 1\n p.x }) }", "E422"},
	{"noalloc fn b4(b: Box[Nm]) -> Int { b.into().n }", ""},
	{"noalloc fn b5(b: Box[Pt]) -> Pt { b.into() }", ""},
	{"noalloc fn b6(b: &Box[Pt]) -> Pt { b.get() }", "E1700"},
	{"noalloc fn b11(b: &Box[Pt], k: Int) -> Int { b.with((p: &Pt) => p.x + k) }", "E910"},
	{"noalloc fn b12(b: &mut Box[Pt], k: Int) -> Int { b.withMut((p: &mut Pt) => { p.x = k\n p.x }) }", "E910"},
	{"noalloc fn b10(b: &Box[Float]) -> Float { b.get() + 1.0 }", ""},
	{"noalloc fn b7(n: Int) -> Int { let b = Box.new(n)\n b.into() }", "E907"},
	{"noalloc fn b8(n: Int) -> Int { match Box.tryNew(n) { Ok(b) -> b.into(), Err(e) -> 0 } }", "E907"},
	{"noalloc fn b9(b: &Box[Nm]) -> Int { b.with((n: &Nm) => mk().n) }", "E422"},
	{"fn c1(n: Int) -> Int { let mut c = 0\n let f = () => { c = n }\n f()\n c }", ""},
	{"noalloc fn c2(n: Int) -> Bool { let f = (x: Int) => x > n\n f(3) }", ""},
	{"noalloc fn c3(xs: &mut Array[Int], k: Int) { xs[1] = xs[0]\n xs[2] = 5 }", ""},
	{"noalloc fn c4(xs: &Array[Int]) -> Int { let mut t = 0\n for x in xs { t = t + x }\n t }", ""},
	{"noalloc fn c5(n: Int) -> Int { let f = (x: Int) => x + n\n f(n + 1) }", "E2126"},
	{"noalloc fn c7(n: Int) -> Int { let f = (x: Int) => x + n\n f(n) }", "E907"},
	{"noalloc fn d0(r: &mut Int, i: Int) { putI(r, i) }", ""},
	{"noalloc fn d1(r: &mut Int) { putI(r, 5000)\n putI(r, 7) }", ""},
	{"noalloc fn d2(f: noalloc fn(Int) -> Bool, i: Int) -> Bool { callI(f, i) }", ""},
	{"noalloc fn d3(r: &mut Int, i: Int) { fwdI(r, i)\n fwdI(r, 3000) }", ""},
	{"noalloc fn d4(r: &mut Int, xs: &mut Array[Int]) { putI(r, xs[0]) }", ""},
	{"noalloc fn d5(r: &mut Int, i: Int) { putI(r, i + 1) }", "E2147"},
	{"noalloc fn d6(r: &mut Int, i: Int) { fwdI(r, i * 2) }", "E2147"},
	{"noalloc fn d7(f: noalloc fn(Int) -> Bool, i: Int) -> Bool { callI(f, i + 1) }", "E2147"},
	{"noalloc fn d8(r: &mut Int, mut i: Int) { i = i + 1\n putI(r, i) }", "E2147"},
	{"noalloc fn d9(p: &mut Pt, i: Int) { putF(p, i.toFloat()) }", ""},
	{"noalloc fn c6(r: &mut Int, n: Int) { r = r + n }", "E2125"},
	{"fn q0(n: Nm) -> Bool { applyGen((x: Int) => take(n) + x > 0) }", "E422"},
	{"fn q1(n: Nm) -> Bool { applyDyn((x: Int) => take(n) + x > 0) }", "E422"},
	{"fn q2(w: W) -> Bool { applyGen((x: Int) => w.nm.n + x > 0) }", ""},
	{"fn q3(w: W) -> Bool { applyGen((x: Int) => { let a = w.nm\n a.n + x > 0 }) }", ""},
	{"fn q4(w: &mut W) -> Bool { applyGen((x: Int) => { w.nm = Nm { name: \"b\", n: x }\n x > 0 }) }", "E422"},
	{"fn q5(w: &mut W) -> Bool { applyDyn((x: Int) => { w.nm = Nm { name: \"b\", n: x }\n x > 0 }) }", "E422"},
}

func TestDeclaredNoallocBodiesCallNoAllocator(t *testing.T) {
	for _, probe := range noallocProbes {
		name, _, _ := strings.Cut(probe.src, "(")
		t.Run(strings.Fields(name)[len(strings.Fields(name))-1], func(t *testing.T) {
			t.Parallel()
			rejected, checked, holes := noallocBodies(t, noallocIRPrelude+probe.src+"\n\nfn main() -> Void! {}\n")
			switch {
			case probe.reject != "" && !strings.Contains(rejected, probe.reject):
				t.Fatalf("want a %s rejection, got %q (holes %v)", probe.reject, rejected, holes)
			case probe.reject == "" && rejected != "":
				t.Fatalf("want it accepted, got:\n%s", rejected)
			case probe.reject == "" && checked < 2:
				t.Fatalf("only %d bodies were audited; the probe and a callee at least must be", checked)
			case len(holes) > 0:
				t.Fatalf("accepted, but a body allocates: %v", holes)
			}
		})
	}
}
