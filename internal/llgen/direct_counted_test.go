package llgen_test

import (
	"strings"
	"testing"
)

const countedIRSrc = `
import {Box} from std/alloc

fn boxed(n: Int) -> Box[Int] {
    Box.new(n)
}

fn odd(n: Int) -> Result[Int, String] {
    if n % 2 == 1 {
        return Err("odd ${n}")
    }
    Ok(n)
}

fn twice(n: Int) -> Result[Int, String] {
    let a = odd(n)?
    odd(a + 1)
}

fn main() -> Void! {
    let b = boxed(3)
    print "${b.get()} ${twice(2) || 0}"
}
`

// A try form writes its Result in the frame: the op hands the payload back bare and nothing wraps it in Ok.
func TestCountedResultBuildsInline(t *testing.T) {
	ir := emitIR(t, countedIRSrc)
	box := bodyOf(t, ir, `define ptr @"std/alloc.Box.new`)
	if !strings.Contains(box, "@rt_d_try(") || strings.Contains(box, "@rt_std_id(") {
		t.Errorf("Box.new does not run tryNew as a bare try op:\n%s", box)
	}
	for _, fn := range []string{`define %"kg.adt.Result[Int,String]" @"main.odd"(`, `define %"kg.adt.Result[Int,String]" @"main.twice"(`} {
		body := bodyOf(t, ir, fn)
		checkNoHeap(t, fn, body)
		if strings.Contains(body, "@rt_payload(") || strings.Contains(body, "@rt_isvariant(") {
			t.Errorf("%s reads its Result through a box:\n%s", fn, body)
		}
	}
	if !strings.Contains(bodyOf(t, ir, `define %"kg.adt.Result[Int,String]" @"main.twice"(`), `@"kg.drop.Result[Int,String]"`) {
		t.Error("twice does not drop the Result it unwrapped through its drop helper")
	}
	drop := bodyOf(t, ir, `define internal void @"kg.drop.Result[Int,String]"(`)
	if strings.Count(drop, "@rt_release(") != 1 {
		t.Errorf("the drop helper releases other than the Err slot's String once:\n%s", drop)
	}
}
