package vm

import (
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

// TestID identifies one test stably across runs as module/package/name, so same-named tests in different packages never collide.
type TestID struct {
	Module  string
	Package string
	Name    string
}

// String renders <package>::<name>, or <module>/<package>::<name> when the package path does not already name the
// module: a module root that is also the only package must not repeat its own name.
func (id TestID) String() string {
	switch {
	case id.Module == "" && id.Package == "":
		return id.Name
	case id.Module == "":
		return id.Package + "::" + id.Name
	case id.Package == id.Module, strings.HasPrefix(id.Package, id.Module+"/"):
		return id.Package + "::" + id.Name
	default:
		return id.Module + "/" + id.Package + "::" + id.Name
	}
}

func testIDOf(prog *mir.Program, module string, f *midir.Func) TestID {
	pkg := prog.R.Packages[prog.R.PackageOf(f.File)].Path
	return TestID{Module: module, Package: pkg, Name: prog.R.Entity(f.Ent).Name}
}
