package verify

import midir "kigumi/internal/compiler/mir/ir"

// A box is reported as a diagnostic; a copy in a `noalloc` body is a compiler fault unless sem charged it, where a declaration already rejected it.
func (p *verifier) verifyAllocClaim(f *midir.Func) *Error {
	if f.Ent == 0 {
		return nil
	}
	declared := p.DeclaredNoalloc(f)
	if !declared && !p.R.Summary(f.Ent).Noalloc {
		return nil
	}
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if !p.AllocCopy(f, in) {
				continue
			}
			if declared && p.copyAccounted(f, in) {
				continue
			}
			return fail("alloc-claim", midir.BlockID(bid), i, "copies %s where the summary claims no allocation", p.R.TypeString(p.CopiedType(f, in)))
		}
	}
	return nil
}

func (p *verifier) copyAccounted(f *midir.Func, in midir.Inst) bool {
	if _, deferred := p.ParamCopy(f, in); deferred {
		return true
	}
	return p.R.CopyCharged(p.CopySite(f, in))
}
