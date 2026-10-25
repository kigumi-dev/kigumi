package llgen_test

import (
	"strings"
	"testing"
)

const primSrc = `
fn narrow(n: Int) -> u16? {
    n.toU16()
}

fn widen(n: u8) -> Int {
    n.toInt() + 1
}

fn byteAt(data: Bytes, i: usize) -> u8 {
    data.get(i) || 0
}

fn pick(xs: &Array[Int], i: usize) -> Int? {
    xs.get(i)
}

fn spin(a: u32, b: u32) -> u32 {
    a.wrappingAdd(b).rotateLeft(b)
}

fn size(s: String) -> usize {
    s.len()
}

fn main() -> Void! {}
`

func TestPrimitiveResultsComeBackWithoutABox(t *testing.T) {
	ir := emitIR(t, primSrc)
	cases := []struct{ fn, call string }{
		{"main.narrow", "icmp ule i64"},
		{"main.widen", "@llvm.sadd.with.overflow.i64("},
		{"main.byteAt", "@rt_d_bytes_get("},
		{"main.pick", "@rt_d_array_get("},
		{"main.size", "@rt_d_len("},
		{"main.spin", "@llvm.fshl.i32("},
	}
	for _, c := range cases {
		body := bodyOf(t, ir, `@"`+c.fn+`"(`)
		if !strings.Contains(body, c.call) {
			t.Errorf("%s does not call %s:\n%s", c.fn, c.call, body)
		}
		for _, bad := range []string{"@rt_std_id(", "@rt_convert(", `@"kg.unbox.`} {
			if strings.Contains(body, bad) {
				t.Errorf("%s still goes through %s:\n%s", c.fn, bad, body)
			}
		}
	}
}

func TestPrimitiveWithAnyOtherResultKeepsTheBoxedCall(t *testing.T) {
	ir := emitIR(t, `
fn names(xs: &Array[String], i: usize) -> String? {
    xs.get(i)
}

fn main() -> Void! {}
`)
	body := bodyOf(t, ir, `@"main.names"(`)
	if strings.Contains(body, "@rt_d_") || !strings.Contains(body, "@rt_std_id(") {
		t.Errorf("an Option of a String must stay on the boxed call:\n%s", body)
	}
}

// Every Map lookup reads the hash seed, so the read must not box the random u64.
func TestMapSeedComesBackWithoutABox(t *testing.T) {
	ir := emitIR(t, `
import map from std/map

fn main() {
    if map.tryEmpty[Int, Int]() is Ok(m) {
        let _ = m.contains(&1)
    }
}
`)
	body := bodyOf(t, ir, `@"std/map.startSlot"(`)
	if !strings.Contains(body, "@rt_d_map_seed(") || strings.Contains(body, "@rt_std_id(") || strings.Contains(body, "@rt_int(") {
		t.Errorf("startSlot boxes the seed:\n%s", body)
	}
}
