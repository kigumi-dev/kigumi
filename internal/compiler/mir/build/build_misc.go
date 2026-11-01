package build

import "kigumi/internal/compiler/sem"

func (b *builder) variantNamed(adt sem.EntityID, name string) sem.EntityID {
	for _, v := range b.r.TypeDecl(adt).Variants {
		if b.r.Entity(v).Name == name {
			return v
		}
	}
	return 0
}
