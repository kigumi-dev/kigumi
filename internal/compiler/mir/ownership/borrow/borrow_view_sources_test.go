package borrow

import (
	"slices"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
)

func TestKeptViewIncomingSources(t *testing.T) {
	p, body := srcProgram(t, `
interface Sink { fn put(mut self, f: Future[usize]) }
type Hold['a] = { pub r &'a mut Array[Future[usize]] }
fn Hold['a].put(mut self, f: Future[usize]) { self.r.push(f) }
fn choose['a](h: &mut Hold['a], j: &mut Hold['a], b: Bool, f: Future[usize]) {
    let v: &mut Sink = if b { h } else { j }
    v.put(f)
}
`)
	g := body("choose")
	v := localNamed(t, g, "v")
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g}
	s := c.viewSources(v)
	var roots []midir.LocalID
	for _, l := range s.locals {
		if r, ok := c.refRoot(l); ok && !slices.Contains(roots, r) {
			roots = append(roots, r)
		}
	}
	slices.Sort(roots)
	if !s.complete || !slices.Equal(roots, g.Params[:2]) {
		t.Fatalf("incoming holders = %v, complete %v; want %v", roots, s.complete, g.Params[:2])
	}
	h := *g
	h.Locals = append(slices.Clone(g.Locals), midir.Local{Type: g.Locals[v].Type})
	unknown := midir.LocalID(len(g.Locals))
	c = &borrowCheck{p: p.input, state: New(p.input, nil), f: &h, viewDefs: allDefs(&h), viewSets: map[midir.LocalID]viewSourceSet{}}
	c.viewDefs[v] = append(c.viewDefs[v], &midir.Inst{Op: midir.OpCopy, Dst: v, Args: []midir.LocalID{v}})
	if s = c.viewSources(v); !s.complete || len(s.locals) != 2 {
		t.Fatalf("a loop edge lost its incoming holders: %+v", s)
	}
	c.viewSets = map[midir.LocalID]viewSourceSet{}
	c.viewDefs[v] = append(c.viewDefs[v], &midir.Inst{Op: midir.OpCopy, Dst: v, Args: []midir.LocalID{unknown}})
	if s = c.viewSources(v); s.complete || !slices.Contains(s.locals, v) || len(c.keptMutTargets(v)) < 3 {
		t.Fatalf("untraced incoming view lost its conservative destination: %+v", s)
	}
	for _, fl := range c.keptMutTargets(v) {
		if fl.target == v {
			if _, _, _, ok := c.storeRoot(fl); !fl.unresolved || ok {
				t.Fatal("an unknown incoming holder resolved to a known alternative")
			}
		}
	}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if !c.ifaceRequirement(in.Ent) {
				continue
			}
			for _, fl := range c.keptFlows(in) {
				if !fl.unresolved {
					continue
				}
				fl.unresolved = false
				if !slices.ContainsFunc(c.withKeptFlows(in, []storeFlow{fl}), func(f storeFlow) bool { return f.unresolved }) {
					t.Fatal("a base flow hid the unknown incoming destination")
				}
				return
			}
		}
	}
	t.Fatal("no unknown incoming flow at the requirement call")
}

func TestKeptUnrecordedRequirementSignature(t *testing.T) {
	p, _ := srcProgram(t, `
interface Absorber { fn absorb(mut self, o: &mut Self) }
fn distinct[T: Absorber](mut x: T, mut y: T) -> T {
    x.absorb(&mut y)
    x
}
`)
	if p.HasErrors() {
		t.Fatal(p.Render())
	}
}
