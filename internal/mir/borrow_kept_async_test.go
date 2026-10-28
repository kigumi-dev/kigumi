package mir

import "testing"

const keptAsyncSrc = `
type Hold['a] = { pub r &'a mut Array[Future[usize]] }
async fn mutInt(c: &mut Int) -> usize { c = c + 1; 1 }
async fn storeFrame(c: &mut Int, h: &mut Array[Future[usize]]) -> usize {
    let f = mutInt
    h.push(f(c))
    1
}
async fn storeHold['a](c: &mut Int, h: &mut Hold['a]) -> usize {
    let f = mutInt
    h.r.push(f(c))
    1
}
async fn countFrame(c: &mut Int, h: &mut Array[Future[usize]]) -> usize {
    c = c + 1
    1
}
async fn countHold['a](c: &mut Int, h: &mut Hold['a]) -> usize {
    c = c + 1
    1
}
`

func TestKeptAsyncStoreSummaries(t *testing.T) {
	p, body := srcProgram(t, keptAsyncSrc)
	for _, tc := range []struct {
		name  string
		store bool
	}{
		{"storeFrame", true},
		{"storeHold", true},
		{"countFrame", false},
		{"countHold", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := body(tc.name)
			c := &borrowCheck{p: p, f: g}
			if s := c.keptBound(g.Ent, nil); s != nil {
				t.Errorf("async construction has a body result summary: %+v", s)
			}
			s := c.keptStoreSummary(g.Ent, nil)
			if s == nil || s.all {
				t.Fatalf("missing precise store summary: %+v", s)
			}
			if s.storesInto(1, 0) != tc.store {
				t.Errorf("storesInto(h, c) = %v, want %v: %+v", s.storesInto(1, 0), tc.store, s)
			}
			if !tc.store {
				for target, bits := range s.stores {
					if bits != 0 {
						t.Errorf("non-storing body keeps values in parameter %d: %b", target, bits)
					}
				}
			}
			if s := c.keptBound(g.Ent, nil); s != nil {
				t.Errorf("cached store summary became an async result summary: %+v", s)
			}
		})
	}
}
