package mir

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

func cliqueSource(prefix string, n int, extra string) string {
	var src strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&src, "type %s%d = { pub n Int", prefix, i)
		for j := 1; j <= n; j++ {
			if j != i {
				fmt.Fprintf(&src, ", pub f%d Array[%s%d]", j, prefix, j)
			}
		}
		if i == n {
			src.WriteString(extra)
		}
		src.WriteString(" }\n")
	}
	return src.String()
}

func memoProgram(t *testing.T, src string) (*Program, func(string) sem.TypeID) {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	pkg := res.PackageByPath("main")
	return &Program{R: res}, func(name string) sem.TypeID { return res.Types.Named(res.PackageMember(pkg, name), nil) }
}

// queryHolds asks c about each name in order and checks it against a fresh typeHolds walk and want.
func queryHolds(t *testing.T, c *borrowCheck, typ func(string) sem.TypeID, names []string, want func(string) bool) {
	t.Helper()
	r := c.p.R
	old := &borrowCheck{p: c.p}
	for _, name := range names {
		oracle := old.typeHolds(typ(name), func(t sem.TypeID) bool {
			return r.Types.Kind(t) == sem.KRef || slices.ContainsFunc(r.Types.Params(t), func(p sem.EntityID) bool { return r.TypeParam(p).IsLifetime })
		})
		if got := c.holdsBorrow(typ(name)); got != oracle || got != want(name) {
			t.Fatalf("%s: holdsBorrow = %v, typeHolds = %v, want %v", name, got, oracle, want(name))
		}
		if m := &c.typeMemo; len(m.scc.stack) != 0 || m.expansions != len(m.decls) {
			t.Fatalf("after %s: %d expansions for %d declarations, %d left open", name, m.expansions, len(m.decls), len(m.scc.stack))
		}
	}
}

func typeNames(prefix string, n int) []string {
	var out []string
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf("%s%d", prefix, i))
	}
	return out
}

func TestHoldsBorrowSettlesMutualRecursionOnce(t *testing.T) {
	const n = 8
	p, typ := memoProgram(t, cliqueSource("T", n, "")+cliqueSource("U", n, ", pub g fn(&String) -> Int"))
	for _, prefix := range []string{"T", "U"} {
		for _, start := range []int{1, n / 2, n} {
			c := &borrowCheck{p: p}
			all := typeNames(prefix, n)
			queryHolds(t, c, typ, append(all[start-1:], all[:start-1]...), func(string) bool { return prefix == "U" })
			if c.typeMemo.expansions > n+8 {
				t.Errorf("%s from %d: %d expansions", prefix, start, c.typeMemo.expansions)
			}
		}
	}
}

// The A cycle reaches the B cycle, where B4 finds a reference with B5 still open above it; the C clique
// hangs off A5 and holds nothing. A lifetime parameter would have to spread around each cycle.
const nestedComponents = `type A1 = { pub n Int, pub next Array[A2] }
type A2 = { pub n Int, pub next Array[A3] }
type A3 = { pub n Int, pub next Array[A4], pub b Array[B1] }
type A4 = { pub n Int, pub next Array[A5] }
type A5 = { pub n Int, pub next Array[A1], pub c Array[C1] }
type B1 = { pub n Int, pub next Array[B2] }
type B2 = { pub n Int, pub next Array[B3] }
type B3 = { pub n Int, pub next Array[B4] }
type B4 = { pub n Int, pub next Array[B5], pub g fn(&String) -> Int }
type B5 = { pub n Int, pub next Array[B1] }
`

func TestHoldsBorrowNestedComponents(t *testing.T) {
	p, typ := memoProgram(t, nestedComponents+cliqueSource("C", 4, ""))
	a, b, cs := typeNames("A", 5), typeNames("B", 5), typeNames("C", 4)
	held := func(name string) bool { return name[0] != 'C' }
	orders := map[string][][]string{
		"A then B then C": {a, b, cs},
		"B before A":      {b, a, cs},
		"deep B5 first":   {{"B5"}, a, b, cs},
		"C before A":      {cs, a, b},
		"A3 first":        {{"A3"}, b, a, cs},
	}
	for name, order := range orders {
		t.Run(name, func(t *testing.T) {
			c := &borrowCheck{p: p}
			for _, group := range order {
				queryHolds(t, c, typ, group, held)
			}
			for _, d := range slices.Concat(a, b, cs) {
				want := declNo
				if held(d) {
					want = declYes
				}
				if got := c.typeMemo.decls[p.R.Types.Node(typ(d)).Ent]; got != want {
					t.Errorf("%s settled as %d, want %d", d, got, want)
				}
			}
		})
	}
}
