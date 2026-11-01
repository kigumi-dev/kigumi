package build

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const refScopeOnlySource = `type V['a] = { pub s &'a String }
fn mk['a](s: &'a String) -> V['a] { V { s: s } }
fn viewLen['a](value: &'a String) -> usize {
    let f: fn(&String) -> V['a] = mk
    f(value).s.len()
}
let text = "hello"
print "${viewLen(&text)}"
`

func checkedSource(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main), IsrContracts: opdef.IsrContracts}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	return res
}

func TestRefScopeCheckReusesTheLoweringAnalyzer(t *testing.T) {
	mk := func(l *lowering) *midir.Func { return l.ByEnt[l.R.PackageMember(l.R.PackageByPath("main"), "mk")] }
	pending := newLowering(checkedSource(t, refScopeOnlySource), nil)
	pending.lowerReachable(false)
	if f := mk(pending); f == nil || pending.borrow.HasFlowSummary(f) {
		t.Fatal("the pending checks must leave mk unsummarized, or this test cannot see the ref-scope check")
	}
	l := lowerChecked(checkedSource(t, refScopeOnlySource), false, nil)
	if !l.borrow.HasFlowSummary(mk(l)) {
		t.Fatal("the ref-scope check summarized mk on an analyzer other than the lowering's")
	}
}
