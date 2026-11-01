package flow

import midir "kigumi/internal/compiler/mir/ir"

// BackwardOrder lists unreachable blocks, then reachable ones in postorder, which settles a backward flow in few sweeps.
func BackwardOrder(f *midir.Func) []midir.BlockID {
	rpo := ReversePostorder(f)
	reached := make([]bool, len(f.Blocks))
	for _, bid := range rpo {
		reached[bid] = true
	}
	var order []midir.BlockID
	for bid := range f.Blocks {
		if !reached[bid] {
			order = append(order, midir.BlockID(bid))
		}
	}
	for i := len(rpo) - 1; i >= 0; i-- {
		order = append(order, rpo[i])
	}
	return order
}
