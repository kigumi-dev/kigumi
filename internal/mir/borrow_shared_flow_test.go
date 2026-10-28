package mir

import (
	"testing"

	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

const sharedFlowSrc = `
import alloc from std/alloc
type Two = { pub a alloc.Shared[Future[usize]?], pub b alloc.Shared[Future[usize]?] }
async fn peek(o: &mut Future[usize]?) -> usize { 3 }
async fn mut1(a: &mut Array[String]) -> usize { a.len() }
fn keepZ(o: &mut Future[usize]?, z: &mut Array[String]) -> Future[usize] { mut1(z) }
fn takenOut(t: Two) {
    let x = t.a.withMut((o: &mut Future[usize]?) => o.take())
    t.b.set(x)
}
fn cellBorrowedOut(t: Two) {
    let f = t.a.withMut(peek)
    t.b.set(Some(f))
}
fn unpacked(t: Two) {
    let x = t.a.withMut((o: &mut Future[usize]?) => o.take())
    if let Some(f) = x {
        t.b.set(Some(f))
    }
}
fn takenFromLocal(t: Two) {
    let mut z = Array.of("a")
    let mut o = Some(mut1(&mut z))
    t.b.set(o.take())
}
fn localBorrowed(t: Two) {
    let mut z = Array.of("a")
    let f = mut1(&mut z)
    let g = f
    t.b.set(Some(g))
}
`

// sharedStoreDiags counts E2158 reports per function of src.
func sharedStoreDiags(t *testing.T, src string) map[string]int {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	out := map[string]int{}
	for _, f := range Build(res).Funcs {
		for _, d := range f.Diags {
			if d.Code == "E2158" {
				out[res.Entity(f.Ent).Name]++
			}
		}
	}
	return out
}

func TestSharedStoreFollowsResultShape(t *testing.T) {
	got := sharedStoreDiags(t, sharedFlowSrc)
	if got["takenOut"] != 0 {
		t.Errorf("takenOut reported %d times: a Future taken out of a cell is owned, not a borrow of t", got["takenOut"])
	}
	if got["unpacked"] != 0 {
		t.Errorf("unpacked reported %d times: the payload of an owned value taken out of a cell is owned too", got["unpacked"])
	}
	if got["cellBorrowedOut"] != 1 {
		t.Errorf("cellBorrowedOut reported %d times, want 1: peek's Future keeps the &mut into t.a's content", got["cellBorrowedOut"])
	}
	if got["takenFromLocal"] != 1 {
		t.Errorf("takenFromLocal reported %d times, want 1: take() hands on what o holds, a Future borrowing z", got["takenFromLocal"])
	}
	if got["localBorrowed"] != 1 {
		t.Errorf("localBorrowed reported %d times, want 1: the Future borrows z through two copies", got["localBorrowed"])
	}
}
