package reach_test

import (
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func baseTypesKept(t *testing.T, src string) map[string]bool {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	built := mirbuild.Program(res)
	rs := mirreach.Reachable(built.R, built.Entry, built.Funcs, built.ByEnt)
	kept := map[string]bool{}
	for _, name := range []string{"Option", "Result", "Map", "Ordering", "Range", "AllocError"} {
		kept[name] = rs.KeepType(entType(t, res, name))
	}
	return kept
}

func TestReachableKeepsBaseTypesOnlyWhenReferenced(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"nothing", "print \"ok\"\n", nil},
		{"range", "let r = 0..3\nprint \"${r.end}\"\n", []string{"Range"}},
		{"ordering", "let o = \"a\".compareTo(&\"b\")\nprint \"${o is Ordering.Less}\"\n", []string{"Ordering"}},
		{"alloc error", "let mut xs = Array.empty[Int]()\nlet r = xs.tryPush(1)\nprint \"${r is Ok(_)}\"\n", []string{"AllocError"}},
		{"map", "import map from std/map\n\nlet mut m = map.empty[Int, Int]()\nm.set(1, 2)\nprint \"${m.len()}\"\n", []string{"Map", "AllocError"}},
	}
	always := map[string]bool{"Option": true, "Result": true}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kept := baseTypesKept(t, c.src)
			want := map[string]bool{}
			for _, n := range c.want {
				want[n] = true
			}
			for name, got := range kept {
				if always[name] {
					continue
				}
				if got != want[name] {
					t.Errorf("%s kept = %v, want %v", name, got, want[name])
				}
			}
		})
	}
}
