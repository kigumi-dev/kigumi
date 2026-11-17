package verify

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

// maxReachNotes bounds the path back toward the root that keeps a body.
const maxReachNotes = 6

// CheckSimdShapes reports each body rs keeps that lowers a SIMD operation whose lanes only an instantiation fixes:
// the native backend lowers such a body only as its instances.
func CheckSimdShapes(p *mir.Program, rs *mirreach.Reachability) []AllocClaim {
	c, ok := sem.CodeByName("simd-shared-body")
	if !ok || rs == nil {
		return nil
	}
	var out []AllocClaim
	for _, f := range p.Funcs {
		if f.Ent == 0 || !rs.Keep(f.Ent) {
			continue
		}
		if claim, open := simdSharedClaim(p, rs, c, f); open {
			out = append(out, claim)
		}
	}
	return out
}

func simdSharedClaim(p *mir.Program, rs *mirreach.Reachability, c sem.Code, f *midir.Func) (AllocClaim, bool) {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			t, open := p.SimdOpen(f, in)
			if !open {
				continue
			}
			tree := p.R.Tree(f.File)
			msg := sem.FillTemplate(c.Template, bodyName(p.R, f.Ent), p.R.TypeString(t))
			d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(tree.File, tree.Span(p.SiteNode(f, in))), Msg: msg, Code: c.Num, Help: []string{c.Help}}
			for _, n := range reachNotes(p, rs, f.Ent) {
				d = d.WithNote(n.Loc, n.Msg)
			}
			return AllocClaim{File: f.File, Diag: d}, true
		}
	}
	return AllocClaim{}, false
}

func reachNotes(p *mir.Program, rs *mirreach.Reachability, ent sem.EntityID) []diag.Diagnostic {
	var out []diag.Diagnostic
	seen := map[sem.EntityID]bool{}
	for len(out) < maxReachNotes && !seen[ent] {
		seen[ent] = true
		why := rs.Reason[ent]
		switch {
		case why.Root == mirreach.RootDropGlue:
			if owner := p.R.Fn(ent).Owner; owner != 0 {
				name := p.R.Entity(owner).Name
				out = appendNote(p, out, owner, "the drop glue of `"+name+"` runs this one body for every `"+name+"`")
			}
			return out
		case why.Root != mirreach.RootNone || why.Caller == 0:
			return out
		case why.Via == "witness":
			out = appendNote(p, out, why.Caller, "`"+bodyName(p.R, why.Caller)+"` builds an interface method table that runs it")
		default:
			out = appendNote(p, out, why.Caller, "`"+bodyName(p.R, why.Caller)+"` reaches it")
		}
		ent = why.Caller
	}
	return out
}

func appendNote(p *mir.Program, out []diag.Diagnostic, at sem.EntityID, msg string) []diag.Diagnostic {
	e := p.R.Entity(at)
	if e.File == 0 {
		return out
	}
	t := p.R.Tree(e.File)
	return append(out, diag.Diagnostic{Loc: diag.At(t.File, t.Span(e.Node)), Msg: msg})
}

func bodyName(r *sem.Result, ent sem.EntityID) string {
	e := r.Entity(ent)
	switch {
	case e.Kind == sem.EntImplicitMain:
		return "main"
	case e.Kind == sem.EntClosure:
		return bodyName(r, e.Parent) + ".closure"
	case e.Kind == sem.EntFn && r.Fn(ent).Owner != 0:
		return r.Entity(r.Fn(ent).Owner).Name + "." + e.Name
	}
	return e.Name
}
