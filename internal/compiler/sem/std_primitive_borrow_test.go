package sem_test

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// primitiveTake is one argument a bodiless std function takes by value per its declaration.
type primitiveTake struct {
	fn  sem.EntityID
	pos int // -1 is the receiver of a method
}

// TestStdPrimitivesRefuseBorrowedArguments: every by-value receiver or parameter of a public
// bodiless std function refuses a field reached through a reference.
func TestStdPrimitivesRefuseBorrowedArguments(t *testing.T) {
	std := semfixture.LoadStd(t, "../../../std")
	res := sem.CheckWith(&sem.Module{Packages: std}, schema.GenerateDerive)
	takes := primitiveTakes(res)
	if len(takes) == 0 {
		t.Fatal("no std primitive takes an argument by value")
	}
	for _, tk := range takes {
		name := primitiveName(res, tk)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := primitiveProbe(res, tk)
			main := &sem.Package{Path: "main"}
			main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
			main.Files = []*syntax.Tree{main.Entry}
			got := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main)}, schema.GenerateDerive)
			span := main.Entry.File.Src
			at := strings.Index(string(span), "h.x")
			for _, d := range got.Diagnostics(main.Entry) {
				if (d.Code == "E2153" || d.Code == "E517") && int(d.Loc.Span.Start) == at {
					return
				}
			}
			t.Errorf("%s: no borrowed-argument error at `h.x`\n%s\n%s", name, src, got.Render())
		})
	}
}

func primitiveTakes(res *sem.Result) []primitiveTake {
	var out []primitiveTake
	for id := 1; id < len(res.Entities); id++ {
		fn := sem.EntityID(id)
		e := res.Entity(fn)
		if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 || e.Vis.Level != sem.VisPub {
			continue
		}
		info := res.Fn(fn)
		if info.Body != 0 || info.Abi != "" || info.Owner != 0 && res.TypeDecl(info.Owner).Drop == fn {
			continue
		}
		if info.Recv == sem.RecvMove && !res.IsCopy(res.Entity(info.SelfParam).Type) {
			out = append(out, primitiveTake{fn, -1})
		}
		for i, p := range info.Params {
			pt := res.Entity(p).Type
			if !res.IsCopy(pt) && res.Types.Kind(pt) != sem.KRef {
				out = append(out, primitiveTake{fn, i})
			}
		}
	}
	return out
}

func primitiveName(res *sem.Result, tk primitiveTake) string {
	info := res.Fn(tk.fn)
	name := res.Entity(tk.fn).Name
	if info.Owner != 0 {
		name = res.Entity(info.Owner).Name + "." + name
	}
	return fmt.Sprintf("%s/%d", name, tk.pos)
}

var typeParamWord = regexp.MustCompile(`\b[A-Z]\w*\b`)

// primitiveProbe calls the primitive with the tested argument read as `h.x` through `h: &Holder`;
// type parameters become a resource so the argument owns what it carries.
func primitiveProbe(res *sem.Result, tk primitiveTake) string {
	info := res.Fn(tk.fn)
	params := map[string]bool{}
	for _, p := range info.TypeParams {
		params[res.Entity(p).Name] = true
	}
	typ := func(ent sem.EntityID) string {
		return typeParamWord.ReplaceAllStringFunc(res.TypeString(res.Entity(ent).Type), func(w string) string {
			if params[w] {
				return "Res"
			}
			return w
		})
	}
	e := res.Entity(tk.fn)
	var b strings.Builder
	imports := map[string][]string{}
	named := map[sem.EntityID]bool{}
	if info.Owner != 0 {
		named[info.Owner] = true
	}
	for _, p := range append([]sem.EntityID{info.SelfParam}, info.Params...) {
		if p != 0 {
			namedTypes(res, res.Entity(p).Type, named)
		}
	}
	for ent := range named {
		if path := res.Packages[res.Entity(ent).Pkg].Path; path != "std/prelude" && !params[res.Entity(ent).Name] {
			imports[path] = append(imports[path], res.Entity(ent).Name)
		}
	}
	callee := e.Name
	if pkg := res.Packages[e.Pkg].Path; info.Owner == 0 && pkg != "std/prelude" {
		fmt.Fprintf(&b, "import p from %s\n", pkg)
		callee = "p." + e.Name
	}
	for path, names := range imports {
		slices.Sort(names)
		fmt.Fprintf(&b, "import {%s} from %s\n", strings.Join(names, ", "), path)
	}
	b.WriteString("\n")
	b.WriteString("type Res = resource {\n    pub id Int\n}\n\nfn Res.drop(move self) {}\n\n")
	held := info.SelfParam
	if tk.pos >= 0 {
		held = info.Params[tk.pos]
	}
	fmt.Fprintf(&b, "type Holder = {\n    pub x %s\n}\n\n", typ(held))
	var decl, args []string
	recv := ""
	if info.Recv != sem.RecvNone {
		recv = "h.x"
		if tk.pos >= 0 {
			recv = "r"
			decl = append(decl, "r: "+typ(info.SelfParam))
		}
	}
	for i, p := range info.Params {
		if i == tk.pos {
			args = append(args, "h.x")
			continue
		}
		arg := fmt.Sprintf("a%d", i)
		decl = append(decl, fmt.Sprintf("%s: %s", arg, typ(p)))
		args = append(args, arg)
	}
	call := callee + "(" + strings.Join(args, ", ") + ")"
	switch {
	case recv != "":
		call = recv + "." + e.Name + "(" + strings.Join(args, ", ") + ")"
	case info.Owner != 0:
		call = res.Entity(info.Owner).Name + "." + call
	}
	fmt.Fprintf(&b, "fn probe(h: &Holder%s) {\n    unsafe {\n        _ = %s\n    }\n}\n", strings.Join(append([]string{""}, decl...), ", "), call)
	return b.String()
}

func namedTypes(res *sem.Result, t sem.TypeID, out map[sem.EntityID]bool) {
	n := res.Types.Node(t)
	if n.Kind == sem.KNamed && n.Ent != 0 {
		out[n.Ent] = true
	}
	if n.Elem != 0 {
		namedTypes(res, n.Elem, out)
	}
	for _, a := range n.Args {
		namedTypes(res, a, out)
	}
}
