package mirfixture

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// Program checks a tiny script so the hand-built functions below have a
// type table and a Result to verify against.
func Program(t *testing.T) *mir.Program {
	t.Helper()
	res := CheckSourceMir(t, "print \"x\"\n")
	return &mir.Program{R: res, ByEnt: map[sem.EntityID]*midir.Func{}}
}

func CheckSourceMir(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, stdDir(t)), main), IsrContracts: opdef.IsrContracts}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	return res
}

// Resource is a move-only type: the Error interface boxed.
func Resource(p *mir.Program) sem.TypeID {
	return p.R.Types.Iface(p.R.Types.ErrorEnt(), nil)
}

func Ret(args ...midir.LocalID) midir.Term { return midir.Term{Op: midir.TermReturn, Args: args} }

func Fn(name string, ret sem.TypeID, locals []midir.Local, blocks ...midir.Block) *midir.Func {
	return &midir.Func{Name: name, Ret: ret, Locals: locals, Blocks: blocks}
}
