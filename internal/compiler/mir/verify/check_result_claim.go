package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// resultClaims names the boxes a body causes in another frame: a boxed
// callee's entry or the callee behind a value or a method table.
func (p *claims) resultClaims(f *midir.Func, code string) []AllocClaim {
	c, ok := sem.CodeByName(code)
	if !ok {
		return nil
	}
	var out []AllocClaim
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if t := p.BoxedResult(f, in); t != 0 {
				out = append(out, p.claimAt(f, c, in.Node, t))
			}
		}
	}
	return out
}
