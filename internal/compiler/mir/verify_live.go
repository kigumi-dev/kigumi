package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// exposedLocals numbers the locals read before a definition in their own block, parameters excepted; only those cross block edges live or undefined.
func exposedLocals(f *midir.Func) (idx []int32, locals []midir.LocalID) {
	idx = make([]int32, len(f.Locals))
	for l := range idx {
		idx[l] = -1
	}
	param := make([]bool, len(f.Locals))
	for _, prm := range f.Params {
		param[prm] = true
	}
	stamp := make([]int32, len(f.Locals))
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		expose := func(l midir.LocalID) {
			if !param[l] && stamp[l] != int32(bid)+1 && idx[l] < 0 {
				idx[l] = int32(len(locals))
				locals = append(locals, l)
			}
		}
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				expose(a)
			}
			if in.Op.Yields() {
				stamp[in.Dst] = int32(bid) + 1
			}
		}
		for _, a := range blk.Term.Args {
			expose(a)
		}
	}
	return idx, locals
}

func (p *verifier) edgeLive(f *midir.Func) *edgeLive {
	body := p.body(f)
	if body.live == nil {
		idx, locals := exposedLocals(f)
		body.live = newEdgeLive(f, idx, locals)
	}
	return body.live
}

type edgeLive struct {
	idx     []int32
	locals  []midir.LocalID
	uses    []wordSet
	defs    []wordSet
	in, out []wordSet
}

func newEdgeLive(f *midir.Func, idx []int32, locals []midir.LocalID) *edgeLive {
	n := len(f.Blocks)
	lv := &edgeLive{idx: idx, locals: locals, uses: make([]wordSet, n), defs: make([]wordSet, n), in: make([]wordSet, n), out: make([]wordSet, n)}
	stamp := make([]int32, len(f.Locals))
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		var used, defined []int32
		use := func(l midir.LocalID) {
			if i := idx[l]; i >= 0 && stamp[l] != int32(bid)+1 {
				used = append(used, i)
			}
		}
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				use(a)
			}
			if in.Op.Yields() {
				stamp[in.Dst] = int32(bid) + 1
				if i := idx[in.Dst]; i >= 0 {
					defined = append(defined, i)
				}
			}
		}
		for _, a := range blk.Term.Args {
			use(a)
		}
		lv.uses[bid], lv.defs[bid] = wordSetOf(used), wordSetOf(defined)
	}
	order := backwardOrder(f)
	for changed := true; changed; {
		changed = false
		for _, bid := range order {
			var out wordSet
			for _, succ := range f.Blocks[bid].Term.Targets {
				out = out.union(lv.in[succ])
			}
			lv.out[bid] = out
			if in := lv.uses[bid].union(out.minus(lv.defs[bid])); !in.equal(lv.in[bid]) {
				lv.in[bid] = in
				changed = true
			}
		}
	}
	return lv
}

// dead calls fn for each local that is live on entry or defined in the block and read on no path after it.
func (lv *edgeLive) dead(f *midir.Func, bid midir.BlockID, fn func(midir.LocalID)) {
	out := lv.out[bid]
	lv.in[bid].minus(out).each(func(i int32) { fn(lv.locals[i]) })
	for _, in := range f.Blocks[bid].Insts {
		if !in.Op.Yields() {
			continue
		}
		// Parameters are excluded from idx even when a later block reads a reassigned value.
		if slices.Contains(f.Params, in.Dst) {
			continue
		}
		if i := lv.idx[in.Dst]; i < 0 || !out.has(i) {
			fn(in.Dst)
		}
	}
}
