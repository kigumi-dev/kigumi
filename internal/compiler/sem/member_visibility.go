package sem

// A member declared outside its type's package is seen only by files that import the declaring package.
func (r *Result) memberVisibleIn(f FileID, m EntityID) bool {
	e := &r.Entities[m]
	owner := r.Fn(m).Owner
	if owner == 0 || e.Pkg == r.packageOf(f) || e.Pkg == r.homeOf(owner) || r.Packages[e.Pkg].Path == "std/prelude" {
		return true
	}
	_, ok := r.Files[f].Imported[e.Pkg]
	return ok
}

// A member reached through an import counts as a use of that import.
func (r *Result) noteMemberImport(f FileID, ent EntityID) {
	e := &r.Entities[ent]
	if e.Kind != EntFn || e.Pkg == r.packageOf(f) || r.Fn(ent).Owner == 0 {
		return
	}
	if imp, ok := r.Files[f].Imported[e.Pkg]; ok {
		r.Entities[imp].Flags |= EfUsed
	}
}

// memberImportedBy is memberVisibleIn for a whole package: some file of it must import the declaring package.
func (r *Result) memberImportedBy(m EntityID, from PackageID) bool {
	e := &r.Entities[m]
	owner := r.Fn(m).Owner
	if owner == 0 || e.Pkg == from || e.Pkg == r.homeOf(owner) || r.Packages[e.Pkg].Path == "std/prelude" {
		return true
	}
	for _, f := range r.Packages[from].Files {
		if imp, ok := r.Files[f].Imported[e.Pkg]; ok {
			r.Entities[imp].Flags |= EfUsed
			return true
		}
	}
	return false
}

func (c *checker) visibleMembers(set OverloadSetID) []EntityID {
	all := c.r.Overloads[set].Members
	out := all[:0:0]
	for _, m := range all {
		if c.r.memberVisibleIn(c.f, m) {
			out = append(out, m)
		}
	}
	return out
}

// memberSetHere is memberSet limited to the members this file can see.
func (c *checker) memberSetHere(t TypeID, name string) OverloadSetID {
	set := c.r.memberSet(t, name)
	if set == 0 {
		return 0
	}
	for _, m := range c.r.Overloads[set].Members {
		if c.r.memberVisibleIn(c.f, m) {
			return set
		}
	}
	return 0
}

// hiddenMemberPackage names the package whose member `name` of t this file does not import, or "".
func (c *checker) hiddenMemberPackage(t TypeID, name string) string {
	if set := c.r.memberSet(t, name); set != 0 {
		return c.r.Packages[c.r.Entities[c.r.Overloads[set].Members[0]].Pkg].Path
	}
	return ""
}

func hiddenMemberHelp(name, pkg string) string {
	return "`" + name + "` is declared in package `" + pkg + "`, which this file does not import; add `import ... from " + pkg + "`"
}
