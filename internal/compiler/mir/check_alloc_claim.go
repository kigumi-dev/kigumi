package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
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
	reached := p.namedCallees()
	p.demoteBoxedLends(reached)
	out = append(out, p.profileClaims()...)
	for _, f := range p.allFuncs() {
		if p.sharedUnreached(f, reached) {
			continue
		}
		box, result := "direct-box-in-noalloc", "result-box-in-noalloc"
		if !p.declaredNoalloc(f) {
			if f.Ent == 0 || !p.R.Summary(f.Ent).Noalloc {
				continue
			}
			box, result = "inferred-box-in-noalloc", "inferred-result-box-in-noalloc"
		}
		for _, c := range append(p.boxClaims(f, box), p.resultClaims(f, result)...) {
			if k := (claimKey{c.Diag.Loc, c.Diag.Msg}); !seen[k] {
				seen[k] = true
				out = append(out, c)
			}
		}
	}
	return out
}

func (p *Program) declaredNoalloc(f *midir.Func) bool {
	if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntFn {
		return false
	}
	return p.R.Fn(f.Ent).Declared&sem.EffNoalloc != 0
}

func (p *Program) boxClaims(f *midir.Func, code string) []AllocClaim {
	declared := p.declaredNoalloc(f)
	c, ok := sem.CodeByName(code)
	if !ok {
		return nil
	}
	var out []AllocClaim
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if typ := p.boxedType(f, in); typ != 0 && (declared || in.Op != midir.OpBorrow) {
				out = append(out, p.claimAt(f, c, in.Node, typ))
			}
		}
	}
	return out
}

func (p *Program) claimAt(f *midir.Func, c sem.Code, n syntax.NodeID, typ sem.TypeID) AllocClaim {
	return p.claimWith(f, c, n, typ, c.Help)
}

func (p *Program) claimWith(f *midir.Func, c sem.Code, n syntax.NodeID, typ sem.TypeID, help string) AllocClaim {
	t := p.R.Tree(f.File)
	msg := sem.FillTemplate(c.Template, p.R.Entity(f.Ent).Name, p.R.TypeString(typ))
	if help != "" {
		msg += "; " + help
	}
	return AllocClaim{File: f.File, Diag: diag.Diagnostic{
		Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Msg: msg, Code: c.Num,
	}}
}

// A box is reported as a diagnostic; a copy in a `noalloc` body is a compiler fault unless sem charged it, where a declaration already rejected it.
func (p *Program) verifyAllocClaim(f *midir.Func) *Error {
	if f.Ent == 0 {
		return nil
	}
	declared := p.declaredNoalloc(f)
	if !declared && !p.R.Summary(f.Ent).Noalloc {
		return nil
	}
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if !p.allocCopy(f, in) {
				continue
			}
			if declared && p.copyAccounted(f, in) {
				continue
			}
			return fail("alloc-claim", midir.BlockID(bid), i, "copies %s where the summary claims no allocation", p.R.TypeString(p.copiedType(f, in)))
		}
	}
	return nil
}

func (p *Program) copyAccounted(f *midir.Func, in midir.Inst) bool {
	if _, deferred := p.paramCopy(f, in); deferred {
		return true
	}
	return p.R.CopyCharged(p.copySite(f, in))
}
