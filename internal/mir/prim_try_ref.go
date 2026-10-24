package mir

import "kigumi/internal/sem"

// tryRefResult: the op hands back its Ok payload bare and a refusal as none, so the Result lives inline;
// a payload that is itself a variant would be ambiguous with the runtime's own Err, unless it is the Option a TryOpt op unwraps too.
func (p *Program) tryRefResult(t sem.TypeID, opt bool) bool {
	ok, err, isRes := p.R.Types.IsResult(t)
	if !isRes || !p.R.DirectAgg(t) || !p.allocError(err) {
		return false
	}
	if opt {
		elem, isOpt := p.R.Types.IsOption(ok)
		return isOpt && p.R.CountedSlot(elem)
	}
	return p.R.CountedSlot(ok) || p.R.RegScalar(ok)
}

func (p *Program) allocError(t sem.TypeID) bool {
	n := p.R.Types.Node(t)
	if n.Kind != sem.KNamed {
		return false
	}
	e := p.R.Entity(n.Ent)
	return e.Name == "AllocError" && p.R.Packages[e.Pkg].Path == "std/prelude"
}
