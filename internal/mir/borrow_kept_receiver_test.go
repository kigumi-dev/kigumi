package mir

import "testing"

func TestKeptReferentDoesNotBorrowReceiver(t *testing.T) {
	p, _ := srcProgram(t, `import map from std/map
fn fill() -> Result[Void, AllocError] {
    let mut m = map.tryEmpty[Int, Int]()?
    m.trySet(3, 0)
}
`)
	if p.HasErrors() {
		t.Fatal(p.Render())
	}
}
