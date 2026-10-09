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

// A body the summary calls `noalloc`, declared or inferred, must not box; `pure` allows allocation.
func CheckAllocClaims(p *Program) []AllocClaim {
	var out []AllocClaim
	// An instantiation reports at its generic body's positions.
	type claimKey struct {
		loc diag.Location
		msg string
	}
	seen := map[claimKey]bool{}
	for _, f := range p.allFuncs() {
		name := "direct-box-in-noalloc"
		if !p.declaredNoalloc(f) {
			if f.Ent == 0 || !p.R.Summary(f.Ent).Noalloc {
				continue
			}
			name = "inferred-box-in-noalloc"
		}
		for _, c := range p.boxClaims(f, name) {
			if k := (claimKey{c.Diag.Loc, c.Diag.Msg}); !seen[k] {
				seen[k] = true
				out = append(out, c)
			}
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

func (p *Program) boxClaims(f *Func, code string) []AllocClaim {
	c, ok := sem.CodeByName(code)
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

// A box is reported as a diagnostic; only a copy in a `noalloc` body is a compiler fault.
func (p *Program) verifyAllocClaim(f *Func) *Error {
	if f.Ent == 0 || !p.R.Summary(f.Ent).Noalloc || p.declaredNoalloc(f) {
		return nil
	}
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if p.allocCopy(f, in) {
				return fail("alloc-claim", BlockID(bid), i, "copies %s where the summary claims no allocation", p.R.TypeString(p.copiedType(f, in)))
			}
		}
	}
	return nil
}
