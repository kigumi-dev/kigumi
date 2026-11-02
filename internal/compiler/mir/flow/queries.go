package flow

import midir "kigumi/internal/compiler/mir/ir"

func SoleDefs(f *midir.Func) map[midir.LocalID]*midir.Inst {
	defs := map[midir.LocalID]*midir.Inst{}
	multi := map[midir.LocalID]bool{}
	for bi := range f.Blocks {
		for ii := range f.Blocks[bi].Insts {
			in := &f.Blocks[bi].Insts[ii]
			if !in.Op.Yields() {
				continue
			}
			if defs[in.Dst] != nil {
				multi[in.Dst] = true
			}
			defs[in.Dst] = in
		}
	}
	for l := range multi {
		delete(defs, l)
	}
	return defs
}

// blockReach[b][b] holds only through a loop.
func BlockReach(blocks []midir.Block) [][]bool {
	out := make([][]bool, len(blocks))
	for b := range blocks {
		seen := make([]bool, len(blocks))
		work := append([]midir.BlockID{}, blocks[b].Term.Targets...)
		for len(work) > 0 {
			c := work[len(work)-1]
			work = work[:len(work)-1]
			if seen[c] {
				continue
			}
			seen[c] = true
			work = append(work, blocks[c].Term.Targets...)
		}
		out[b] = seen
	}
	return out
}

func SoleDef(f *midir.Func, l midir.LocalID) *midir.Inst {
	var def *midir.Inst
	for _, blk := range f.Blocks {
		for k := range blk.Insts {
			if d := &blk.Insts[k]; d.Op.Yields() && d.Dst == l {
				if def != nil {
					return nil
				}
				def = d
			}
		}
	}
	return def
}

func ReversePostorder(f *midir.Func) []midir.BlockID {
	seen := make([]bool, len(f.Blocks))
	var post []midir.BlockID
	type frame struct {
		bid  midir.BlockID
		next int
	}
	stack := []frame{{0, 0}}
	seen[0] = true
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		targets := f.Blocks[top.bid].Term.Targets
		if top.next < len(targets) {
			succ := targets[top.next]
			top.next++
			if !seen[succ] {
				seen[succ] = true
				stack = append(stack, frame{succ, 0})
			}
			continue
		}
		post = append(post, top.bid)
		stack = stack[:len(stack)-1]
	}
	for i, j := 0, len(post)-1; i < j; i, j = i+1, j-1 {
		post[i], post[j] = post[j], post[i]
	}
	return post
}

func Preds(f *midir.Func) [][]midir.BlockID {
	counts := make([]int, len(f.Blocks))
	edges := 0
	for b := range f.Blocks {
		for _, t := range f.Blocks[b].Term.Targets {
			counts[t]++
			edges++
		}
	}
	all := make([]midir.BlockID, edges)
	preds := make([][]midir.BlockID, len(f.Blocks))
	off := 0
	for b, n := range counts {
		preds[b] = all[off : off : off+n]
		off += n
	}
	for b := range f.Blocks {
		for _, t := range f.Blocks[b].Term.Targets {
			preds[t] = append(preds[t], midir.BlockID(b))
		}
	}
	return preds
}
