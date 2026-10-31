package sem_test

import (
	"math/rand"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
)

// A declaration's place in the file must not change what is reported in it.
func TestRootBridgeDeclarationOrder(t *testing.T) {
	for _, name := range []string{"async_root_bridge_flow", "async_root_bridge_value", "async_root_bridge_scope"} {
		t.Run(name, func(t *testing.T) {
			fx := parseFixture(t, "../../../testdata/sem/"+name+".txtar")
			idx := -1
			for i, s := range fx.sections {
				if s.name == "app/t/t.kg" {
					idx = i
				}
			}
			if idx < 0 {
				t.Fatal("no app/t/t.kg section")
			}
			head, blocks := splitDecls(fx.sections[idx].body)
			want := reportedIn(t, fx, idx, head, blocks)
			if len(want) == 0 {
				t.Fatal("the fixture reports nothing")
			}
			orders := [][]int{reverse(len(blocks)), rotate(len(blocks), 7)}
			for seed := int64(1); seed <= 4; seed++ {
				orders = append(orders, rand.New(rand.NewSource(seed)).Perm(len(blocks)))
			}
			for _, order := range orders {
				permuted := make([]string, len(blocks))
				for i, j := range order {
					permuted[i] = blocks[j]
				}
				if got := reportedIn(t, fx, idx, head, permuted); !slices.Equal(got, want) {
					t.Fatalf("order %v reports\n%v\nwant\n%v", order, got, want)
				}
			}
		})
	}
}

func reverse(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = n - 1 - i
	}
	return out
}

func rotate(n, by int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = (i + by) % n
	}
	return out
}

// splitDecls keeps the imports in place and cuts the rest into blocks at blank lines.
func splitDecls(src string) (string, []string) {
	parts := strings.Split(strings.TrimSpace(src), "\n\n")
	head := ""
	if strings.HasPrefix(parts[0], "import ") {
		head, parts = parts[0], parts[1:]
	}
	return head, parts
}

// reportedIn runs the check on the declarations in the given order and names, per E1291, the declaration it falls in.
func reportedIn(t *testing.T, fx fixture, idx int, head string, blocks []string) []string {
	t.Helper()
	body := head + "\n\n" + strings.Join(blocks, "\n\n") + "\n"
	start := make([]int, len(blocks))
	line := strings.Count(head, "\n") + 3
	for i, b := range blocks {
		start[i] = line
		line += strings.Count(b, "\n") + 2
	}
	mod := fx
	mod.sections = slices.Clone(fx.sections)
	mod.sections[idx].body = body
	res := sem.CheckWith(buildModule(t, mod), schema.GenerateDerive)
	var out []string
	for i := 1; i < len(res.Files); i++ {
		for _, d := range res.Diagnostics(res.Files[i].Tree) {
			if d.Code != "E1291" {
				continue
			}
			src := res.Source(d.Loc.Source).Src
			at := strings.Count(string(src[:d.Loc.Span.Start]), "\n") + 1
			for b := len(blocks) - 1; b >= 0; b-- {
				if at >= start[b] {
					out = append(out, firstLine(blocks[b])+" | "+d.Msg)
					break
				}
			}
		}
	}
	slices.Sort(out)
	return out
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}
