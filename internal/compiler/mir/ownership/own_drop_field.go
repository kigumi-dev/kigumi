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
				if d := a.p.R.TypeDecl(node.Ent).Drop; d != 0 && d != a.f.Ent {
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
