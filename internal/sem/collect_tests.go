package sem

import "kigumi/internal/syntax"

func (r *Result) noteTestName(f FileID, d syntax.NodeID, id EntityID) {
	pkg := &r.Packages[r.packageOf(f)]
	name := r.Entities[id].Name
	if prev, dup := pkg.tests[name]; dup {
		r.errNote(f, d, Binding{Ent: prev}, "the first test with this name is here", cTestNameDuplicate, name)
		return
	}
	if pkg.tests == nil {
		pkg.tests = map[string]EntityID{}
	}
	pkg.tests[name] = id
}
