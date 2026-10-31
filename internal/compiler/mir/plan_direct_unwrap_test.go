package mir_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
)

const unwrapSrc = `
type In = {
    pub mut a Int
    pub mut b Int
}

fn try1(i: In) -> Result[In, Int] {
    if i.a > 0 { Ok(In { a: i.a + 1, b: i.b }) } else { Err(i.b) }
}

fn fresh(i: In) -> Result[Int, Int] {
    let mut x = try1(i)?
    x.a = 5
    Ok(x.a)
}

fn main() -> Void! {}
`

func TestPlanDirectKeepsAMutatedUnwrappedPayloadUnboxed(t *testing.T) {
	p := specialized(t, unwrapSrc)
	if n := countOp(funcNamed(t, p, "main.fresh"), midir.OpBoxDirect); n != 0 {
		t.Errorf("unwrapping a call's Result into a mutable local boxes %d times:\n%s", n, p.Dump())
	}
}
