package llgen_test

import (
	"strings"
	"testing"
)

const protoProbe = `type Point = {
    pub x Int
    pub y Float
}

type Tag = On(n: Int, ok: Bool) | Off

fn same(a: Point, b: Point) -> Bool { a == b }

fn fingerprint(a: Point) -> u64 { hash(&a) }

fn sameTag(a: Tag, b: Tag) -> Bool { a == b }

fn tagHash(a: Tag) -> u64 { hash(&a) }

print "${same(Point { x: 1, y: 2.5 }, Point { x: 1, y: 2.5 })} ${fingerprint(Point { x: 1, y: 2.5 })} ${sameTag(On(1, true), Off)} ${tagHash(Off)}"
`

func TestDirectComparisonAndHashNeverBox(t *testing.T) {
	ir := specializedIR(t, protoProbe)
	for _, c := range []struct{ fn, call string }{
		{"main.same", `call i1 @"kg.eq.main.Point"(`},
		{"main.fingerprint", `call i64 @"kg.hash.main.Point"(`},
		{"main.sameTag", `call i1 @"kg.eq.main.Tag"(`},
		{"main.tagHash", `call i64 @"kg.hash.main.Tag"(`},
	} {
		body := fnBody(t, ir, c.fn)
		if !strings.Contains(body, c.call) {
			t.Errorf("%s does not call %s:\n%s", c.fn, c.call, body)
		}
		for _, bad := range []string{`@"kg.box.`, `@"kg.unbox.`, "@rt_record(", "@rt_variant(", "@rt_std_id(", `@"main.Point.equals"`, `@"main.Tag.equals"`} {
			if strings.Contains(body, bad) {
				t.Errorf("%s calls %s:\n%s", c.fn, bad, body)
			}
		}
	}
	for _, fn := range []string{`define internal i1 @"kg.eq.main.Point"(`, `define internal i1 @"kg.eq.main.Tag"(`, `define internal i64 @"kg.hash.main.Point"(`, `define internal i64 @"kg.hash.main.Tag"(`} {
		body := bodyOf(t, ir, fn)
		for _, bad := range []string{"@rt_int(", "@rt_float(", "@rt_record(", "@rt_variant(", "@rt_field(", "@rt_std_id("} {
			if strings.Contains(body, bad) {
				t.Errorf("%s calls %s:\n%s", fn, bad, body)
			}
		}
	}
	if !strings.Contains(bodyOf(t, ir, `define internal i64 @"kg.hash.main.Point"(`), "@rt_d_hash_float(") {
		t.Error("the hash of a Float leaf does not use the runtime's text hash")
	}
}
