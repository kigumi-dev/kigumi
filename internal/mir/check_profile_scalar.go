package mir

import (
	"fmt"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// ArgBox is a call argument whose box the native lowering makes in the caller, at the call or where the value is made.
type ArgBox struct {
	Block BlockID
	Inst  int
	Arg   int
}

// argBoxClaims reports a scalar a trusted user body hands to a boxed parameter, whose box the native lowering makes in that body.
func (p *Program) argBoxClaims(out []AllocClaim, reached []profileReach, c sem.Code) []AllocClaim {
	if p.ArgBoxes == nil {
		panic("mir: a profile check needs Program.ArgBoxes, the native lowering's call-site scalar boxes")
	}
	boxes := p.ArgBoxes(p)
	for _, r := range reached {
		f := r.f
		if !p.userBody(f) || !p.profileTrusts(f.Ent) {
			continue
		}
		for _, ab := range boxes[f] {
			in := f.Blocks[ab.Block].Insts[ab.Inst]
			n := argNode(f, ab, in.Node)
			if p.R.InAllocatorBlock(f.File, n) {
				continue
			}
			help := fmt.Sprintf("`%s` of `%s` takes it boxed, so pass a constant or a value that is already boxed, or allocate inside an `allocator` block", p.paramName(in.Ent, ab.Arg), p.calleeLabel(in.Ent))
			out = append(out, p.claimWith(f, c, n, f.Locals[in.Args[ab.Arg]].Type, help))
		}
	}
	return out
}

// paramName names the parameter argument i fills; a method's receiver comes first.
func (p *Program) paramName(ent sem.EntityID, i int) string {
	if g := p.ByEnt[ent]; g != nil && i < len(g.Params) {
		return g.Locals[g.Params[i]].Name
	}
	if p.R.Entity(ent).Kind == sem.EntFn {
		info := p.R.Fn(ent)
		params := info.Params
		if info.SelfParam != 0 {
			params = append([]sem.EntityID{info.SelfParam}, params...)
		}
		if i < len(params) {
			return p.R.Entity(params[i]).Name
		}
	}
	return fmt.Sprintf("argument %d", i+1)
}

// argNode is the expression that made the argument, through the copies the call itself made, or the call when its block holds none.
func argNode(f *Func, ab ArgBox, call syntax.NodeID) syntax.NodeID {
	insts := f.Blocks[ab.Block].Insts
	a := insts[ab.Inst].Args[ab.Arg]
	for i := ab.Inst - 1; i >= 0; i-- {
		in := insts[i]
		if !in.Op.Yields() || in.Dst != a {
			continue
		}
		if in.Node != 0 && in.Node != call {
			return in.Node
		}
		if !copyLike(in.Op) && in.Op != OpCopy {
			break
		}
		a = in.Args[0]
	}
	return call
}

func (p *Program) calleeLabel(ent sem.EntityID) string {
	if inst, ok := p.R.InstanceOf(ent); ok {
		ent = inst.Gen
	}
	if id := p.R.DeclID(ent); !id.IsZero() {
		return id.Path
	}
	return p.R.Entity(ent).Name
}
