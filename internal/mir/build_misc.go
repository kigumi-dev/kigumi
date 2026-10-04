package mir

import "kigumi/internal/sem"

func (b *builder) variantNamed(adt sem.EntityID, name string) sem.EntityID {
	for _, v := range b.r.TypeDecl(adt).Variants {
		if b.r.Entity(v).Name == name {
			return v
		}
	}
	return 0
}

// closureOrdinal numbers a closure among its parent's closures in
// declaration order, so names stay stable across unrelated edits.
func (p *Program) closureOrdinal(ent sem.EntityID) int {
	parent := p.R.Entity(ent).Parent
	n := 0
	for id := sem.EntityID(1); id <= ent; id++ {
		if e := p.R.Entity(id); e.Kind == sem.EntClosure && e.Parent == parent {
			n++
		}
	}
	return n
}
