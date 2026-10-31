package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// collect (pass 1) creates entities for every top-level declaration and fills
// package scopes; signatures and bodies resolve later.
func (r *Result) collect() {
	for id := 1; id < len(r.Packages); id++ {
		for _, f := range r.Packages[id].Files {
			r.collectFile(f)
		}
	}
	for id := 1; id < len(r.Packages); id++ {
		r.collectEntry(PackageID(id))
	}
	if pre, ok := r.pathIndex["std/prelude"]; ok {
		r.fillPrelude(pre)
	}
}

// fillPrelude makes the pub declarations of std/prelude visible in every
// file without an import.
func (r *Result) fillPrelude(pkg PackageID) {
	for name, b := range r.Scopes[r.Packages[pkg].Scope].Names {
		vis := Visibility{Level: VisPub}
		if b.Ent != 0 {
			vis = r.Entities[b.Ent].Vis
		} else if len(r.Overloads[b.Set].Members) > 0 {
			vis = r.Entities[r.Overloads[b.Set].Members[0]].Vis
		}
		if vis.Level == VisPub {
			r.Scopes[r.prelude].Names[name] = b
		}
	}
}

func (r *Result) collectFile(f FileID) {
	t := r.tree(f)
	for _, d := range r.topDecls(f) {
		switch t.Kind(d) {
		case syntax.FnDecl:
			r.collectFn(f, d, "")
		case syntax.TypeDecl:
			r.collectType(f, d, false)
		case syntax.InterfaceDecl:
			s := ifaceDecl(t, d)
			if uni := r.lookupUniverse(t.TokText(s.Name)); uni != 0 && r.Packages[r.packageOf(f)].Std && r.Entities[uni].Kind == EntInterface {
				r.adoptUniverseIface(f, d, uni)
				continue
			}
			id := r.declareTop(f, d, s.Name, EntInterface, r.resolveVis(f, s.Vis, r.packageOf(f)))
			if id != 0 {
				r.Entities[id].Detail = r.addIface(InterfaceInfo{})
			}
		case syntax.ConstDecl:
			s := constDecl(t, d)
			id := r.declareTop(f, d, s.Name, EntConst, r.resolveVis(f, s.Vis, r.packageOf(f)))
			if id != 0 {
				r.Entities[id].Detail = r.addConst(ConstInfo{})
			}
		case syntax.StaticDecl:
			r.collectStatic(f, d, StaticOwned, "")
		case syntax.ContextKeyDecl:
			s := contextKeyDecl(t, d)
			r.declareTop(f, d, s.Name, EntContextKey, r.resolveVis(f, s.Vis, r.packageOf(f)))
		case syntax.TestDecl:
			td := t.View(d).AsTestDecl()
			id := r.newEntity(Entity{Kind: EntTest, Name: td.Name().Text(), Pkg: r.packageOf(f), File: f, Node: d, Tok: td.Tok().Index()})
			r.Entities[id].Detail = r.addFn(FnInfo{Body: td.Body().ID()})
			r.Files[f].Defs[d] = id
			r.noteTestName(f, d, id)
		case syntax.AbiBlock:
			r.collectAbi(f, d)
		}
	}
}

func (r *Result) collectAbi(f FileID, block syntax.NodeID) {
	t := r.tree(f)
	blk := t.View(block).AsAbiBlock()
	abiArgs := blk.Abis()
	abi := abiArgs.At(0).Tok().Text()
	export := blk.Tok().Kind() != token.KwExtern
	if abi == "interrupt" && !export {
		r.errAt(f, abiArgs.ItemID(0), cInterruptExtern)
	} else if abi != "C" && abi != "interrupt" {
		r.errAt(f, abiArgs.ItemID(0), cAbiUnknown, abi)
	}
	noalloc, naked := false, false
	for i, av := range abiArgs.Enumerate() {
		if i == 0 {
			continue
		}
		a := av.ID()
		switch av.Tok().Text() {
		case "noalloc":
			noalloc = true
		case "naked":
			naked = true
			if !export {
				r.errAt(f, a, cNakedExtern)
			}
		}
	}
	for d := range blk.Decls().IDs() {
		switch t.Kind(d) {
		case syntax.FnDecl:
			if export {
				r.collectFn(f, d, "")
				if id := r.Files[f].Defs[d]; id != 0 {
					r.Fn(id).Export = abi
					r.Fn(id).Naked = naked
				}
				continue
			}
			r.collectFn(f, d, abi)
			if id := r.Files[f].Defs[d]; id != 0 && noalloc {
				r.Fn(id).Declared |= EffNoalloc
			}
		case syntax.TypeDecl:
			r.collectType(f, d, true)
		case syntax.StaticDecl:
			if export {
				r.collectStatic(f, d, StaticExport, abi)
			} else {
				r.collectStatic(f, d, StaticExtern, abi)
			}
		}
	}
}

func (r *Result) collectFn(f FileID, d syntax.NodeID, abi string) {
	t := r.tree(f)
	s := fnDecl(t, d)
	pkg := r.packageOf(f)
	name := t.TokText(s.Name)
	e := Entity{Kind: EntFn, Name: name, Pkg: pkg, File: f, Node: d, Tok: s.Name, Vis: r.resolveVis(f, s.Vis, pkg)}
	if t.Toks[s.Name].Kind != token.Ident {
		e.Flags |= EfOperator
	}
	if r.Packages[pkg].Std {
		e.Flags |= EfStd
	}
	info := FnInfo{Body: s.Body, Declared: Effects(s.Mods), Abi: abi}
	if s.Recv != 0 {
		id := r.newEntity(e)
		r.Entities[id].Detail = r.addFn(info)
		r.Files[f].Defs[d] = id
		return
	}
	if !r.checkTopName(f, d, name) {
		return
	}
	id := r.newEntity(e)
	r.Entities[id].Detail = r.addFn(info)
	r.Files[f].Defs[d] = id
	scope := r.Packages[pkg].Scope
	if prev, dup := r.define(scope, name, Binding{}); dup {
		if prev.Set == 0 {
			r.errNote(f, d, prev, "`"+name+"` was first declared here", cNameRedeclared, name, r.Packages[pkg].Path)
			r.Entities[id].Flags |= EfPoison
			return
		}
		r.Overloads[prev.Set].Members = append(r.Overloads[prev.Set].Members, id)
		r.Fns[r.Entities[id].Detail].Set = prev.Set
		return
	}
	set := r.addOverloadSet(OverloadSet{Name: name, Pkg: pkg, Members: []EntityID{id}})
	r.Scopes[scope].Names[name] = Binding{Set: set}
	r.Fns[r.Entities[id].Detail].Set = set
}
