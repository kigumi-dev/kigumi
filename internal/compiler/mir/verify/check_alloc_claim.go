package verify

import (
	"kigumi/internal/compiler/mir"
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
// The lend demotion runs first because it settles the summaries the claims read.
func CheckAllocClaims(p *mir.Program) []AllocClaim {
	c := &claims{Program: p}
	return c.allocClaims(c.demoteBoxedLends())
}

func (p *claims) allocClaims(reached map[sem.EntityID]bool) []AllocClaim {
	var out []AllocClaim
	// An instantiation reports at its generic body's positions.
	type claimKey struct {
		loc diag.Location
		msg string
	}
	seen := map[claimKey]bool{}
	out = append(out, p.profileClaims()...)
	for _, f := range p.AllFuncs() {
		if p.SharedUnreached(f, reached) {
			continue
		}
		box, result := "direct-box-in-noalloc", "result-box-in-noalloc"
		if !p.DeclaredNoalloc(f) {
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

func (p *claims) boxClaims(f *midir.Func, code string) []AllocClaim {
	declared := p.DeclaredNoalloc(f)
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

func (p *claims) claimAt(f *midir.Func, c sem.Code, n syntax.NodeID, typ sem.TypeID) AllocClaim {
	return p.claimWith(f, c, n, typ, c.Help)
}

func (p *claims) claimWith(f *midir.Func, c sem.Code, n syntax.NodeID, typ sem.TypeID, help string) AllocClaim {
	t := p.R.Tree(f.File)
	msg := sem.FillTemplate(c.Template, p.R.Entity(f.Ent).Name, p.R.TypeString(typ))
	if help != "" {
		msg += "; " + help
	}
	return AllocClaim{File: f.File, Diag: diag.Diagnostic{
		Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Msg: msg, Code: c.Num,
	}}
}
