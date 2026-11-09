package ownership

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// checkDropOwner rejects taking place l when it lies in a field of a type with a custom drop, which would later see the taken value.
func (a *owner) checkDropOwner(n syntax.NodeID, l midir.LocalID, report bool) {
	if !report {
		return
	}
	for c := l; ; {
		par, ok := a.pl.parent[c]
		if !ok {
			return
		}
		if !a.pl.payloadStep(c) {
			t := a.f.Locals[par].Type
			if node := a.p.R.Types.Node(t); node.Kind == sem.KNamed && node.Ent != 0 && a.p.R.Entity(node.Ent).Kind == sem.EntType {
				if d := a.p.R.TypeDecl(node.Ent).Drop; d != 0 && !((d == a.f.Ent || a.moveSelfOf(node.Ent)) && a.dropSelfRoot(par)) {
					c, _ := sem.CodeByName("move-drop-type")
					name := a.ownerName(l)
					msg, help := sem.FillCode(c, name, a.p.R.TypeString(t), name)
					if help != "" {
						msg += "; " + help
					}
					a.report(n, c.Num, msg)
					return
				}
			}
		}
		c = par
	}
}

// checkArmTake applies checkDropOwner to a field place copied out as the value of an `if` or `match` arm.
func (a *owner) checkArmTake(in *midir.Inst, report bool) {
	if in.Op != midir.OpCopy || in.Guarded || in.Share || a.f.Locals[in.Dst].Ent != 0 || !a.pl.isPlace(in.Args[0]) {
		return
	}
	if l := a.pl.canon(in.Args[0]); a.tracked[l] {
		a.checkDropOwner(a.defNode[in.Args[0]], l, report)
	}
}

func (pl *places) payloadStep(l midir.LocalID) bool {
	k := pl.key[l]
	i := strings.LastIndexByte(k, '.')
	return i >= 0 && strings.HasPrefix(k[i+1:], "p")
}

// dropSelfRoot: place l is rooted at the `self` of the destructor being lowered, the one value whose
// own fields that destructor takes apart.
func (a *owner) dropSelfRoot(l midir.LocalID) bool {
	for {
		par, ok := a.pl.parent[l]
		if !ok {
			break
		}
		l = par
	}
	return len(a.f.Params) > 0 && l == a.f.Params[0]
}

func (a *owner) moveSelfOf(ent sem.EntityID) bool {
	return a.p.R.Entity(a.f.Ent).Kind == sem.EntFn && a.p.R.Fn(a.f.Ent).Recv == sem.RecvMove && a.p.R.Fn(a.f.Ent).Owner == ent
}
