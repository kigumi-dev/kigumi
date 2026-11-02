package sem_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const holderPrelude = `fn blen['a](x: &'a Array[Int]) -> usize { x.len() }
pub interface Sized2 { fn size(self) -> usize }
pub type R2[F: Fn[fn(&Array[Int]) -> usize] + Copy] = { pub cb F }
pub fn R2[F].size(self) -> usize {
    let xs = Array.of(1, 2, 3)
    (self.cb)(&xs)
}
pub type Hn = { pub n Int }
fn Hn.measure[R: Sized2](self, r: R) -> usize { r.size() }
`

// TestHolderProofScale pins that a holder's calls are proved once per method, not per value it can hold, so the proof
// count grows linearly with reassignments and with an alias chain instead of with their product.
func TestHolderProofScale(t *testing.T) {
	for _, tc := range []struct {
		name string
		body func(n int) string
	}{
		{"reassign", func(n int) string {
			var b strings.Builder
			b.WriteString("    let mut g = hn.measure\n")
			for range n {
				b.WriteString("    x = x + g(ra)\n    g = hn.measure\n")
			}
			return b.String()
		}},
		{"chain", func(n int) string {
			var b strings.Builder
			b.WriteString("    let mut g0 = hn.measure\n")
			for i := 1; i < n; i++ {
				fmt.Fprintf(&b, "    let mut g%d = hn.measure\n    g%d = g%d\n", i, i, i-1)
			}
			for i := range n {
				fmt.Fprintf(&b, "    x = x + g%d(ra)\n", i)
			}
			return b.String()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const n = 200
			src := holderPrelude + "fn probe['a](hn: Hn, xs: &'a Array[Int]) -> usize {\n" +
				"    let ra: R2[fn(&'a Array[Int]) -> usize] = R2 { cb: blen }\n    let mut x: usize = 0\n" +
				tc.body(n) + "    x\n}\nlet hn = Hn { n: 1 }\nlet xs = Array.of(9)\nprint \"${probe(hn, &xs)}\"\n"
			main := &sem.Package{Path: "main"}
			main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
			main.Files = []*syntax.Tree{main.Entry}
			res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main)}, schema.GenerateDerive)
			if res.HasErrors() {
				t.Fatalf("a safe holder program must pass:\n%s", res.Render())
			}
			if uses := len(res.RefScopeUses()); uses > 4*n {
				t.Fatalf("%d erasure proofs for %d calls: a holder's calls must be proved once per method", uses, n)
			}
		})
	}
}
