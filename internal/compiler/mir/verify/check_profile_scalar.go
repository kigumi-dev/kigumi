package verify

import (
	"fmt"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

func (p *claims) scalarBoxClaims(out []AllocClaim, reached []profileReach, c sem.Code) []AllocClaim {
	if p.NativeBoxes == nil {
		panic("mir: a profile check needs Program.NativeBoxes, the native lowering's call-site scalar boxes")
	}
	boxes := p.NativeBoxes(p.Program)
	resultCode := "pointer-result-in-recoverable-only"
	if c.Name == "box-without-default-allocator" {
		resultCode = "pointer-result-without-default-allocator"
	}
	rc, _ := sem.CodeByName(resultCode)
	opCode := "primitive-box-in-recoverable-only"
	if c.Name == "box-without-default-allocator" {
		opCode = "primitive-box-without-default-allocator"
	}
	oc, _ := sem.CodeByName(opCode)
	seen := map[blamedBox]bool{}
	for _, r := range reached {
		f := r.f
		if f.Ent == 0 || !p.profileTrusts(f.Ent) {
			continue
		}
		for _, rb := range boxes.Results[f] {
			in := f.Blocks[rb.Block].Insts[rb.Inst]
			in.Type = f.Locals[in.Dst].Type
			out = p.blameBox(out, seen, r, rc, in)
		}
		for _, ob := range boxes.Operations[f] {
			in := f.Blocks[ob.Block].Insts[ob.Inst]
			in.Type = ob.Type
			out = p.blameBox(out, seen, r, oc, in)
		}
		if !p.userBody(f) {
			continue
		}
		for _, ab := range boxes.Arguments[f] {
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
func (p *claims) paramName(ent sem.EntityID, i int) string {
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
func argNode(f *midir.Func, ab mir.ArgBox, call syntax.NodeID) syntax.NodeID {
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
		if in.Op != midir.OpAlias && in.Op != midir.OpShare && in.Op != midir.OpMove && in.Op != midir.OpCopy {
			break
		}
		a = in.Args[0]
	}
	return call
}

func (p *claims) calleeLabel(ent sem.EntityID) string {
	if inst, ok := p.R.InstanceOf(ent); ok {
		ent = inst.Gen
	}
	if id := p.R.DeclID(ent); !id.IsZero() {
		return id.Path
	}
	return p.R.Entity(ent).Name
}
