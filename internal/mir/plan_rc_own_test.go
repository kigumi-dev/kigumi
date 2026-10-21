package mir_test

import (
	"testing"

	"kigumi/internal/mir"
)

const ownSrc = `
type Named = {
    pub name String
    pub mut n Int
}

type Holder = {
    pub mut q Named
    pub k Int
}

fn Holder.qOf(self) -> Named {
    self.q
}

fn setN(mut x: Named) -> Int {
    x.n = 5
    x.n
}

fn readN(x: Named) -> Int {
    x.n
}

fn written(h: Holder) -> Int {
    let mut b = h.q
    b.n = 2
    b.n + h.q.n
}

fn readOnly(h: Holder) -> Int {
    let b = h.q
    b.n + h.k
}

fn rootWritten(mut h: Holder) -> Int {
    let b = h.q
    h.q.n = 3
    b.n
}

fn pushThenWrite() -> Int {
    let mut r = Named { name: "a", n: 1 }
    let mut rs: Array[Named] = Array.empty()
    rs.push(r)
    r.n = 7
    rs[0].n
}

fn pushOnly() -> Int {
    let r = Named { name: "a", n: 1 }
    let mut rs: Array[Named] = Array.empty()
    rs.push(r)
    rs[0].n
}

fn passes(h: Holder) -> Int {
    setN(h.q) + readN(h.q)
}

fn getter(h: Holder) -> Int {
    let mut q = h.qOf()
    q.n = 9
    q.n + h.q.n
}

fn main() -> Void! {}
`

func rcPlanned(t *testing.T, src string) *mir.Program {
	t.Helper()
	prog := mir.Build(checkSourceMir(t, src))
	if prog.HasErrors() {
		t.Fatalf("build errors:\n%s", prog.Render())
	}
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatalf("plan-rc: %v\n%s", err, prog.Dump())
	}
	return prog
}

func copies(f *mir.Func) [][2]mir.Inst {
	var out [][2]mir.Inst
	for _, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op == mir.OpShare && in.Copy && f.Locals[in.Args[0]].Ent == 0 && i+1 < len(blk.Insts) {
				out = append(out, [2]mir.Inst{in, blk.Insts[i+1]})
			}
		}
	}
	return out
}

func TestPlanRCCopiesABindingAWriteCanReach(t *testing.T) {
	prog := rcPlanned(t, ownSrc)
	for name, want := range map[string]int{"main.written": 1, "main.readOnly": 0, "main.rootWritten": 1, "main.getter": 1} {
		f := funcNamed(t, prog, name)
		cs := copies(f)
		if len(cs) != want {
			t.Fatalf("%s: %d deep copies of a read, want %d\n%s", name, len(cs), want, prog.Dump())
		}
		for _, c := range cs {
			if f.Locals[c[0].Dst].Ent == 0 || c[1].Op != mir.OpRelease || c[1].Args[0] != c[0].Args[0] {
				t.Fatalf("%s: the copy must bind the named local and hand the read's reference back at once, got %v then %v", name, c[0], c[1])
			}
		}
	}
}

func TestPlanRCCopiesAValueAPrimitiveMayKeep(t *testing.T) {
	prog := rcPlanned(t, ownSrc)
	for name, want := range map[string]int{"main.pushThenWrite": 1, "main.pushOnly": 0} {
		f := funcNamed(t, prog, name)
		n := 0
		for _, blk := range f.Blocks {
			for i, in := range blk.Insts {
				if in.Op != mir.OpCall || prog.R.Entity(in.Ent).Name != "push" {
					continue
				}
				arg := in.Args[len(in.Args)-1]
				prev, next := blk.Insts[i-1], blk.Insts[i+1]
				if prev.Op == mir.OpShare && prev.Copy && prev.Dst == arg {
					n++
					if next.Op != mir.OpRelease || next.Args[0] != arg {
						t.Fatalf("%s: the copy handed to push must be released after it, got %v", name, next)
					}
				}
			}
		}
		if n != want {
			t.Fatalf("%s: push takes %d copies, want %d\n%s", name, n, want, prog.Dump())
		}
	}
}

func TestPlanRCCopiesAnArgumentTheCalleeWrites(t *testing.T) {
	prog := rcPlanned(t, ownSrc)
	f := funcNamed(t, prog, "main.passes")
	got := map[string]bool{}
	for _, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op != mir.OpCall {
				continue
			}
			prev := blk.Insts[i-1]
			got[prog.R.Entity(in.Ent).Name] = prev.Op == mir.OpShare && prev.Copy && prev.Dst == in.Args[0]
		}
	}
	if !got["setN"] || got["readN"] {
		t.Fatalf("setN writes its parameter and readN does not, so only setN takes a copy: %v\n%s", got, prog.Dump())
	}
}

const repeatSrc = `
fn repeat[T: Copy](value: T, n: Int) -> Array[T] {
    let mut out: Array[T] = Array.empty()
    let mut i = 0
    for i < n {
        out.push(value)
        i = i + 1
    }
    out
}

fn main() -> Void! {}
`

func TestPlanRCCopiesATypeParameterOnlyIfWritable(t *testing.T) {
	prog := rcPlanned(t, repeatSrc)
	f := funcNamed(t, prog, "main.repeat")
	for _, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op != mir.OpCall || prog.R.Entity(in.Ent).Name != "push" {
				continue
			}
			prev := blk.Insts[i-1]
			if prev.Op != mir.OpShare || !prev.Copy || !prev.IfWritable || prev.Dst != in.Args[1] {
				t.Fatalf("push must take a copy-mut of the type parameter, got %v\n%s", prev, prog.Dump())
			}
			return
		}
	}
	t.Fatalf("no push in repeat\n%s", prog.Dump())
}

const insideSrc = `
type Named = {
    pub name String
    pub mut n Int
}

type W = {
    pub mut o Named?
}

fn kept(w: W) -> Int {
    let mut x = w.o.unwrapOr(Named { name: "d", n: 0 })
    x.n = 5
    x.n + w.o.unwrapOr(Named { name: "d", n: 0 }).n
}

fn dead(w: W) -> Int {
    let mut x = w.o.unwrapOr(Named { name: "d", n: 0 })
    x.n = 5
    x.n
}

fn main() -> Void! {}
`

func TestPlanRCCopiesAnOptionHoldingAWritableRecord(t *testing.T) {
	prog := rcPlanned(t, insideSrc)
	for name, want := range map[string]bool{"main.kept": true, "main.dead": false} {
		f := funcNamed(t, prog, name)
		var first *mir.Inst
		var prev mir.Inst
		for _, blk := range f.Blocks {
			for i, in := range blk.Insts {
				if in.Op == mir.OpCall && prog.R.Entity(in.Ent).Name == "unwrapOr" && first == nil {
					first, prev = &blk.Insts[i], blk.Insts[i-1]
				}
			}
		}
		if first == nil {
			t.Fatalf("%s: no unwrapOr call\n%s", name, prog.Dump())
		}
		got := prev.Op == mir.OpShare && prev.Copy && !prev.IfWritable && prev.Dst == first.Args[0]
		if got != want {
			t.Fatalf("%s: unwrapOr takes a copy = %v, want %v\n%s", name, got, want, prog.Dump())
		}
	}
}
