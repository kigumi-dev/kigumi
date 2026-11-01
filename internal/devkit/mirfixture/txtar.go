package mirfixture

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

type section struct{ name, body string }

func parseTxtar(src string) []section {
	var out []section
	var cur *section
	for _, line := range strings.SplitAfter(src, "\n") {
		trimmed := strings.TrimRight(line, "\n")
		if strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") {
			out = append(out, section{name: strings.TrimSuffix(strings.TrimPrefix(trimmed, "-- "), " --")})
			cur = &out[len(out)-1]
			continue
		}
		if cur != nil {
			cur.body += line
		}
	}
	return out
}

// CheckFixture loads a fixture's packages over std and type-checks them;
// the raw text and the reassembled source sections come back for goldens.
func CheckFixture(t *testing.T, path string, std []*sem.Package) (*sem.Result, string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pkgs := map[string]*sem.Package{}
	for _, p := range std {
		pkgs[p.Path] = p
	}
	var head string
	for _, s := range parseTxtar(string(data)) {
		switch s.name {
		case "expect", "stdout", "stderr", "exit", "engines":
			continue
		}
		if s.name != "src" && (!strings.HasSuffix(s.name, ".kg") || filepath.Base(s.name) == "mod.kg") {
			continue
		}
		name := s.name
		if name == "src" {
			name = "main/main.kg"
		}
		tree := syntax.Parse(token.NewFile(name, []byte(s.body)))
		if tree.HasErrors() {
			t.Fatalf("%s: parse errors:\n%s", name, tree.Dump())
		}
		dir := filepath.ToSlash(filepath.Dir(name))
		p := pkgs[dir]
		if p == nil {
			p = &sem.Package{Path: dir}
			pkgs[dir] = p
		}
		p.Files = append(p.Files, tree)
		if name == "main/main.kg" {
			p.Entry = tree
		}
		head += "-- " + s.name + " --\n" + s.body
	}
	mod := &sem.Module{IsrContracts: opdef.IsrContracts}
	var paths []string
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		mod.Packages = append(mod.Packages, pkgs[p])
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	return res, string(data), head
}

// UserDump leaves the std bodies, and the closures inside them, out of the
// golden output.
func UserDump(p *mir.Program) string {
	user := &mir.Program{R: p.R, ByEnt: p.ByEnt}
	for _, f := range p.Funcs {
		root := f.Ent
		for p.R.Entity(root).Kind == sem.EntClosure {
			root = p.R.Entity(root).Parent
		}
		if p.R.Entity(root).Flags&sem.EfStd == 0 {
			user.Funcs = append(user.Funcs, f)
		}
	}
	return user.Dump()
}
