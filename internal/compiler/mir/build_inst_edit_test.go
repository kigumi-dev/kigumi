package mir

import (
	"slices"
	"testing"
)

func markedFunc() (*Func, *ghostSet) {
	block := func(ids ...int) Block {
		var b Block
		for _, id := range ids {
			b.Insts = append(b.Insts, Inst{Op: OpNop, Index: id})
		}
		return b
	}
	f := &Func{Blocks: []Block{block(0, 1, 2, 3), block(10, 11)}}
	g := &ghostSet{}
	g.runs = []ghostRun{{marks: []markPos{{0, 0}, {0, 2}, {0, 4}}}, {marks: []markPos{{1, 1}, {1, 0}}}}
	return f, g
}

// markedAt is the Index of the instruction each mark stands before, -1 for a mark at its block's end.
func markedAt(f *Func, g *ghostSet) []int {
	var out []int
	for _, r := range g.runs {
		for _, m := range r.marks {
			insts := f.Blocks[m.blk].Insts
			if m.idx == len(insts) {
				out = append(out, -1)
				continue
			}
			out = append(out, insts[m.idx].Index)
		}
	}
	return out
}

func TestInstEditKeepsGhostMarks(t *testing.T) {
	added := Inst{Op: OpNop, Index: 99}
	cases := []struct {
		name string
		edit func(f *Func, g *ghostSet)
		want []int
	}{
		{"insert at block start", func(f *Func, g *ghostSet) { g.insertInst(f, 0, 0, added) }, []int{0, 2, -1, 11, 10}},
		{"insert before a marked instruction", func(f *Func, g *ghostSet) { g.insertInst(f, 0, 2, added) }, []int{0, 2, -1, 11, 10}},
		{"insert after a marked instruction", func(f *Func, g *ghostSet) { g.insertInst(f, 0, 3, added) }, []int{0, 2, -1, 11, 10}},
		{"insert at block end", func(f *Func, g *ghostSet) { g.insertInst(f, 0, 4, added) }, []int{0, 2, -1, 11, 10}},
		{"insert in another block", func(f *Func, g *ghostSet) { g.insertInst(f, 1, 1, added) }, []int{0, 2, -1, 11, 10}},
		{"delete before a marked instruction", func(f *Func, g *ghostSet) { g.deleteInst(f, 0, 1) }, []int{0, 2, -1, 11, 10}},
		{"delete after a marked instruction", func(f *Func, g *ghostSet) { g.deleteInst(f, 0, 3) }, []int{0, 2, -1, 11, 10}},
		{"delete the first instruction", func(f *Func, g *ghostSet) { g.deleteInst(f, 0, 0) }, []int{1, 2, -1, 11, 10}},
		{"delete a marked instruction", func(f *Func, g *ghostSet) { g.deleteInst(f, 0, 2) }, []int{0, 3, -1, 11, 10}},
		{"delete the last instruction", func(f *Func, g *ghostSet) { g.deleteInst(f, 1, 1) }, []int{0, 2, -1, -1, 10}},
		{"move within a block", func(f *Func, g *ghostSet) { g.insertInst(f, 0, 1, g.deleteInst(f, 0, 3)) }, []int{0, 2, -1, 11, 10}},
		{"move across blocks", func(f *Func, g *ghostSet) { g.insertInst(f, 1, 0, g.deleteInst(f, 0, 2)) }, []int{0, 3, -1, 11, 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, g := markedFunc()
			tc.edit(f, g)
			if got := markedAt(f, g); !slices.Equal(got, tc.want) {
				t.Fatalf("marks stand before %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDropGhostsKeepsBlocksAfterThem(t *testing.T) {
	jump := func(to BlockID) Term { return Term{Op: TermJump, Targets: []BlockID{to}} }
	f := &Func{
		Locals: make([]Local, 5),
		Blocks: []Block{
			{Term: Term{Op: TermBranch, Targets: []BlockID{3, 4}}},
			{Term: jump(2)},
			{Term: Term{Op: TermUnreachable}},
			{Insts: []Inst{{Op: OpNop, Index: 7}}, Term: jump(4)},
			{Term: Term{Op: TermReturn}},
		},
	}
	g := &ghostSet{runs: []ghostRun{{entry: 1, end: 2}, {entry: 2, end: 3}}, blocks: 1, locals: 3}
	g.drop(f)
	if len(f.Blocks) != 3 || len(f.Locals) != 3 || len(g.runs) != 0 {
		t.Fatalf("got %d blocks, %d locals, %d runs; want 3, 3, 0", len(f.Blocks), len(f.Locals), len(g.runs))
	}
	if got := f.Blocks[0].Term.Targets; !slices.Equal(got, []BlockID{1, 2}) {
		t.Fatalf("entry targets %v, want [1 2]", got)
	}
	if f.Blocks[1].Insts[0].Index != 7 || !slices.Equal(f.Blocks[1].Term.Targets, []BlockID{2}) || f.Blocks[2].Term.Op != TermReturn {
		t.Fatalf("the block after the ghosts was not kept and renumbered: %+v", f.Blocks)
	}
}
