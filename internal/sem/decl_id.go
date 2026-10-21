package sem

// DeclID names a declaration by spelling alone, never by load order.
type DeclID struct {
	// Module is the dependency the package came from, "" for the module being built.
	Module  string
	Package string
	Path    string
	Sig     string
}

// IsZero reports an entity that has no declaration identity (a local, a parameter).
func (d DeclID) IsZero() bool { return d.Path == "" }

func (d DeclID) String() string {
	s := d.Path
	if d.Package != "" {
		s = d.Package + "." + s
	}
	if d.Sig != "" {
		s += "#" + d.Sig
	}
	return s
}

// DeclID is the stable identity of ent; an instance of a generic function shares its generic's.
func (r *Result) DeclID(ent EntityID) DeclID {
	if inst, ok := r.instances[ent]; ok {
		ent = inst.Gen
	}
	path, ok := r.declPath(ent)
	if !ok {
		return DeclID{}
	}
	e := r.Entities[ent]
	id := DeclID{Package: r.Packages[e.Pkg].Path, Module: r.Packages[e.Pkg].Module, Path: path}
	switch e.Kind {
	case EntFn:
		if of := r.Fn(ent).DefaultOf; of != 0 {
			id.Sig = r.overloadSig(of)
		} else {
			id.Sig = r.overloadSig(ent)
		}
	case EntClosure:
		id.Sig = r.DeclID(e.Parent).Sig
	}
	return id
}

func (r *Result) declPath(ent EntityID) (string, bool) {
	if ent == 0 || int(ent) >= len(r.Entities) {
		return "", false
	}
	e := r.Entities[ent]
	switch e.Kind {
	case EntType, EntAlias, EntInterface, EntConstraint, EntConst, EntStatic, EntContextKey, EntIntrinsic, EntContract:
		return e.Name, true
	case EntFn:
		if of := r.Fn(ent).DefaultOf; of != 0 {
			path, ok := r.declPath(of)
			return path + "$default." + r.Fn(ent).DefaultParam, ok
		}
		if owner := r.Fn(ent).Owner; owner != 0 {
			return r.ownerPath(owner, e.Pkg) + "." + e.Name, true
		}
		return e.Name, true
	case EntVariant, EntField:
		return r.ownerPath(e.Parent, e.Pkg) + "." + e.Name, true
	case EntTest:
		return "test." + e.Name, true
	case EntImplicitMain:
		return "main." + r.Tree(e.File).File.Name, true
	case EntClosure:
		if r.closure(ent).IsMethodValue() {
			return "", false
		}
		parent, _ := r.declPath(e.Parent)
		if parent != "" {
			parent += "."
		}
		return parent + "closure" + itoa(r.ClosureOrdinal(ent)), true
	}
	return "", false
}

// ownerPath qualifies an owner declared in another package than the member.
func (r *Result) ownerPath(owner EntityID, pkg PackageID) string {
	o := r.Entities[owner]
	if o.Pkg != pkg && r.Packages[o.Pkg].Path != "" {
		return r.Packages[o.Pkg].Path + "." + o.Name
	}
	return o.Name
}
