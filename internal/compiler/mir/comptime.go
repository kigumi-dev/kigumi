package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// ComptimeFunc is one `comptime { ... }` block: its body as a function, and
// the placeholder instruction in Owner that Fold turns into the constant.
type ComptimeFunc struct {
	Site  sem.ComptimeSite
	Func  *midir.Func
	Owner *midir.Func
	Block midir.BlockID
	// Slot is the placeholder's Inst.Index; passes before Fold shift plain indices, so Fold finds it by identity.
	Slot int
	// Type is the block's type at the use site, which the constant takes.
	Type sem.TypeID
	// Cloned marks the copy a specialised instance made of a generic body's block.
	Cloned bool
}

// Fold records the evaluated value with the checker and replaces the
// block's placeholder with a constant, so every back end sees a literal.
func (p *Program) Fold(cf *ComptimeFunc, lit sem.Literal) {
	p.R.SetLiteral(cf.Site.File, cf.Site.Node, lit)
	insts := cf.Owner.Blocks[cf.Block].Insts
	for i := range insts {
		in := &insts[i]
		if in.Op == midir.OpBuiltin && in.Str == "comptime" && in.Index == cf.Slot {
			*in = midir.Inst{Op: midir.OpConst, Dst: in.Dst, Lit: lit, Type: cf.Type, Node: in.Node}
			return
		}
	}
	panic("mir: Fold: placeholder not found")
}
