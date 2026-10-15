package llgen_test

import (
	"strings"
	"testing"
)

const resultProbe = `
type Point = {
    pub x Int
    pub y Int
}

fn half(n: Int) -> Result[Int,Int] {
    if n % 2 == 0 { Ok(n / 2) } else { Err(n) }
}

fn step(n: Int) -> Result[Int,Int] {
    let a = half(n)?
    Ok(a + 1)
}

fn spot(i: Int) -> Result[Point, Int] {
    if i % 2 == 0 { Ok(Point { x: i, y: 1 }) } else { Err(i) }
}

fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        match step(i) {
            Ok(v) -> { s = s + v }
            Err(e) -> { s = s - e }
        }
        if spot(i) is Ok(p) { s = s + p.x }
    }
    s
}

fn main() -> Unit! {
    if run(3) > 100 {}
}
`

func TestResultOfDirectPayloadsIsReturnedUnboxed(t *testing.T) {
	ir := specializedIR(t, resultProbe)
	for _, header := range []string{
		`define %"kg.adt.Result[Int,Int]" @"main.half"(`,
		`define %"kg.adt.Result[Int,Int]" @"main.step"(`,
		`define void @"main.spot"(ptr %sret,`,
		`define i64 @"main.run"(`,
	} {
		body := bodyOf(t, ir, header)
		checkNoHeap(t, header, body)
		for _, bad := range []string{`@"kg.box.`, `@"kg.unbox.`, "@rt_std_id(", "@rt_isvariant("} {
			if strings.Contains(body, bad) {
				t.Errorf("%s calls %s:\n%s", header, bad, body)
			}
		}
	}
}
