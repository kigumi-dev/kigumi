package driver_test

import (
	"fmt"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

var entityKindNames = map[sem.EntityKind]string{
	sem.EntFn: "fn", sem.EntType: "type", sem.EntVariant: "variant",
	sem.EntClosure: "closure", sem.EntStatic: "static", sem.EntAlias: "alias",
}

func entityKindByName(name string) (sem.EntityKind, bool) {
	for k, n := range entityKindNames {
		if n == name {
			return k, true
		}
	}
	return 0, false
}

// entityKey names an entity by its package, its parent and its own name, which survive a change in how many entities the compiler makes.
func entityKey(p *mir.Program, e sem.EntityID) string {
	ent := p.R.Entity(e)
	parent := ""
	if ent.Parent != 0 {
		parent = p.R.Entity(ent.Parent).Name
	}
	return fmt.Sprintf("%s|%s|%s", p.R.Packages[ent.Pkg].Path, parent, ent.Name)
}

func nameOfEntity(p *mir.Program, e sem.EntityID) (key string, nth int) {
	key = entityKey(p, e)
	kind := p.R.Entity(e).Kind
	for i := range p.R.Entities {
		id := sem.EntityID(i)
		if id == e {
			return key, nth
		}
		if p.R.Entity(id).Kind == kind && entityKey(p, id) == key {
			nth++
		}
	}
	return key, nth
}

func findEntity(p *mir.Program, pk pick) sem.EntityID {
	seen := 0
	for i := range p.R.Entities {
		id := sem.EntityID(i)
		if p.R.Entity(id).Kind != pk.ekind || entityKey(p, id) != pk.text {
			continue
		}
		if seen == pk.nth {
			return id
		}
		seen++
	}
	panic(staleRecipe(fmt.Sprintf("no %s entity %q#%d", entityKindNames[pk.ekind], pk.text, pk.nth)))
}

func nameOfType(p *mir.Program, t sem.TypeID) (text string, nth int) {
	text = p.R.TypeString(t)
	for i := 0; i < int(t); i++ {
		if p.R.TypeString(sem.TypeID(i)) == text {
			nth++
		}
	}
	return text, nth
}

func findType(p *mir.Program, pk pick) sem.TypeID {
	seen := 0
	for i := 0; i < p.R.Types.Len(); i++ {
		if p.R.TypeString(sem.TypeID(i)) != pk.text {
			continue
		}
		if seen == pk.nth {
			return sem.TypeID(i)
		}
		seen++
	}
	panic(staleRecipe(fmt.Sprintf("no type %q#%d", pk.text, pk.nth)))
}

func nameOfLocal(f *mir.Func, id int) pick {
	if id >= len(f.Locals) {
		return pick{kind: 'l', oob: true, nth: id - len(f.Locals)}
	}
	name, nth := f.Locals[id].Name, 0
	for i := 0; i < id; i++ {
		if f.Locals[i].Name == name {
			nth++
		}
	}
	return pick{kind: 'l', text: name, nth: nth}
}
