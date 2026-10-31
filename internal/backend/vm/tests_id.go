package vm

import "kigumi/internal/compiler/mir"

// TestID identifies one test stably across runs as module/package/name, so same-named tests in different packages never collide.
type TestID struct {
	Module  string
	Package string
	Name    string
}

func (id TestID) String() string {
	switch {
	case id.Module == "" && id.Package == "":
		return id.Name
	case id.Module == "":
		return id.Package + "::" + id.Name
	default:
		return id.Module + "/" + id.Package + "::" + id.Name
	}
}

func testIDOf(prog *mir.Program, module string, f *mir.Func) TestID {
	pkg := prog.R.Packages[prog.R.PackageOf(f.File)].Path
	return TestID{Module: module, Package: pkg, Name: prog.R.Entity(f.Ent).Name}
}
