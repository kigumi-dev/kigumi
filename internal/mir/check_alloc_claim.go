package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
)

// AllocClaim is one diagnostic the alloc-claim check produced, with the
// file it belongs to: the driver files them once the passes have run.
type AllocClaim struct {
	File sem.FileID
	Diag diag.Diagnostic
}

// Only a declared `noalloc` is checked, not an inferred one: an inferred effect is
// the effect pass's own guess, not the author's promise. `pure` allows allocation.
func CheckAllocClaims(p *Program) []AllocClaim {
	var out []AllocClaim
	for _, f := range p.allFuncs() {
		if p.declaredNoalloc(f) {
			out = append(out, p.boxClaims(f)...)
		}
	}
	return out
}

func (p *Program) declaredNoalloc(f *Func) bool {
	if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntFn {
		return false
	}
	return p.R.Fn(f.Ent).Declared&sem.EffNoalloc != 0
}

func (p *Program) boxClaims(f *Func) []AllocClaim {
	c, ok := sem.CodeByName("direct-box-in-noalloc")
	if !ok {
		return nil
	}
	t := p.R.Tree(f.File)
	var out []AllocClaim
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != OpBoxDirect {
				continue
			}
			msg := sem.FillTemplate(c.Template, p.R.Entity(f.Ent).Name, p.R.TypeString(in.Type))
			if c.Help != "" {
				msg += "; " + c.Help
			}
			out = append(out, AllocClaim{File: f.File, Diag: diag.Diagnostic{
				Severity: diag.Error, Loc: diag.At(t.File, t.Span(in.Node)), Msg: msg, Code: c.Num,
			}})
		}
	}
	return out
}
