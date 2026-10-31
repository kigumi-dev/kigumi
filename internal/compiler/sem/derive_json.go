package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// Derived methods (toJson/fromJson/equals/hash) are
// generated as Kigumi source so both backends run ordinary code with no
// type metadata at runtime; std types implement them by hand instead.

type deriveRequest struct {
	decl  EntityID
	iface string // "Encode", "Decode", "Eq", "Hash" or "Display"
}

// deriveMethods names the method each derivable interface requires.
var deriveMethods = map[string]string{"Encode": "toJson", "Decode": "fromJson", "Eq": "equals", "Hash": "hash", "Display": "display"}

type generatedFile struct {
	pkg  int
	path string
	name string
	text string
}

// deriveIface names a std package's own `@derivable` interface with
// exactly one requirement, or ""; a non-std interface never qualifies.
func (r *Result) deriveIface(iface TypeID) string {
	if r.Types.Kind(iface) != KIface {
		return ""
	}
	ent := r.Types.Node(iface).Ent
	e := &r.Entities[ent]
	if !r.Packages[e.Pkg].Std {
		return ""
	}
	info := r.iface(ent)
	if !info.Derivable || len(info.Reqs) != 1 {
		return ""
	}
	return e.Name
}

// requestDerive records that t needs a generated method for iface and
// reports whether one can be generated.
func (r *Result) requestDerive(t, iface TypeID) bool {
	name := r.deriveIface(iface)
	if name == "" || r.mod.derived {
		return false
	}
	n := r.Types.Node(t)
	if n.Kind != KNamed || (r.Packages[r.Entities[n.Ent].Pkg].Std && !isProto(name)) {
		return false
	}
	if form := r.typeDecl(n.Ent).Form; form != FormRecord && form != FormAdt && !(form == FormResource && !isProto(name)) {
		return false
	}
	for _, d := range r.derives {
		if d.decl == n.Ent && d.iface == name {
			return true
		}
	}
	r.derives = append(r.derives, deriveRequest{decl: n.Ent, iface: name})
	return true
}

func isProto(iface string) bool { return iface == "Eq" || iface == "Hash" }

// deriveSources generates one JSON file, one protocol file and one display
// file per package, covering the requested types and their fields transitively.
func (r *Result) deriveSources() []generatedFile {
	g := &deriver{r: r, gen: r.mod.deriveGen, done: map[deriveRequest]bool{}, texts: map[PackageID]string{}, protos: map[PackageID]string{}, displays: map[PackageID]string{}}
	for _, d := range r.derives {
		g.derive(d)
	}
	var out []generatedFile
	for pkg := 1; pkg < len(r.Packages); pkg++ {
		dir := ""
		for _, f := range r.Packages[pkg].Files {
			dir = dirOf(r.Files[f].Tree.File.Name)
			break
		}
		path := r.Packages[pkg].Path
		if text, ok := g.texts[PackageID(pkg)]; ok {
			out = append(out, generatedFile{pkg: r.moduleIndex(PackageID(pkg)), path: path, name: dir + "json.derived.kg", text: "import json from std/json\n" + text})
		}
		if text, ok := g.protos[PackageID(pkg)]; ok {
			out = append(out, generatedFile{pkg: r.moduleIndex(PackageID(pkg)), path: path, name: dir + "proto.derived.kg", text: text})
		}
		if text, ok := g.displays[PackageID(pkg)]; ok {
			out = append(out, generatedFile{pkg: r.moduleIndex(PackageID(pkg)), path: path, name: dir + "display.derived.kg", text: text})
		}
	}
	return out
}

func (r *Result) moduleIndex(pkg PackageID) int {
	for i, p := range r.mod.Packages {
		if p.Path == r.Packages[pkg].Path {
			return i
		}
	}
	return 0
}

func dirOf(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			return name[:i+1]
		}
	}
	return ""
}

// Check runs CheckWith with noDeriveGenerator: a module needing one derived
// fails with E1432; call CheckWith directly with a real generator instead.
func Check(mod *Module) *Result {
	return CheckWith(mod, noDeriveGenerator)
}

// CheckWith is Check with an explicit DeriveGenerator (internal/toolchain/contract/schema's,
// in the real pipeline).
func CheckWith(mod *Module, gen DeriveGenerator) *Result {
	mod.deriveGen = gen
	r := check(mod)
	if r.Cancelled() || len(r.derives) == 0 || mod.derived {
		return r
	}
	mod.derived = true
	generated := r.deriveSources()
	// A generator failure (E1430, E1432) belongs to this pass; the rerun
	// below would just bury it behind a generic missing-witness error instead.
	if r.genFailed {
		return r
	}
	for _, g := range generated {
		f := token.NewFile(g.name, []byte(g.text))
		f.Generated = true
		mod.Packages[g.pkg].Files = append(mod.Packages[g.pkg].Files, syntax.Parse(f))
	}
	r2 := check(mod)
	if r2.Cancelled() {
		return r2
	}
	// The rerun's own r2.derives is empty (requestDerive no-ops once
	// mod.derived is set), so provenance carries the first pass's answer.
	r2.derives, r2.generated = r.derives, generated
	return r2
}
