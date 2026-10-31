package mir

import (
	"slices"
	"strings"
	"testing"
)

const captureFlowSrc = `
type V['a] = { pub s &'a String }
type Sink['a] = { pub mut items &'a mut Array[V['a]] }
type Own['a] = { pub mut own Array[V['a]] }
fn apply['a](g: fn(V['a]) -> Void, x: V['a]) { g(x) }
fn f['a](x: V['a]) -> usize {
    let loc = "frame-local"
    let mut c = Own { own: Array.of(x) }
    BODY
    c.own.len()
}
`

func TestImmediateCaptureFollowsValue(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"direct", "let r = &mut c.own", ""},
		{"record", "let s = Sink { items: &mut c.own }; let r = s.items", ""},
		{"taken", "let mut arr = Array.of(Sink { items: &mut c.own }); if let Some(s) = arr.takeAt(0) { let r = s.items", "}"},
		{"unwrap", "let mut arr = Array.of(Sink { items: &mut c.own }); let s = arr.takeAt(0).unwrap(); let r = s.items", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.ReplaceAll(captureFlowSrc, "BODY", tc.before+"; apply((y: V['a]) => { r.push(y) }, V { s: &loc })"+tc.after)
			p, body := srcProgram(t, src)
			g := body("f")
			c := collected(p, g)
			loc, dst := localNamed(t, g, "loc"), localNamed(t, g, "c")
			if !slices.ContainsFunc(c.holderOwners(dst), func(o ownerRef) bool { return o.owner == loc }) {
				t.Error("destination lost the stored borrow")
			}
			if got := checkImmediateCaptures(p, g); len(got) != 0 {
				t.Errorf("stored borrow conflicts with immediate capture: %v", got)
			}
		})
	}
}

func TestImmediateCaptureKeepsBorrowedOwner(t *testing.T) {
	src := strings.ReplaceAll(captureFlowSrc, "BODY", "let r = &mut loc; apply((y: V['a]) => { _ = r.len() }, V { s: &loc })")
	src = strings.ReplaceAll(src, "let loc", "let mut loc")
	p, body := srcProgram(t, src)
	if got := checkImmediateCaptures(p, body("f")); len(got) == 0 {
		t.Error("capture lost the reference's actual owner")
	}
}

func TestImmediateCaptureKeepsCallResultOwner(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"taken", "let mut arr = Array.of(Sink { items: &mut c.own }); if let Some(s) = arr.takeAt(0) { let r = s.items", "}"},
		{"unwrap", "let mut arr = Array.of(Sink { items: &mut c.own }); let s = arr.takeAt(0).unwrap(); let r = s.items", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.ReplaceAll(captureFlowSrc, "BODY", tc.before+"; apply((y: V['a]) => { r.push(y) }, V { s: &loc })"+tc.after)
			p, body := srcProgram(t, src)
			g := body("f")
			c := collected(p, g)
			r, loc, dst := localNamed(t, g, "r"), localNamed(t, g, "loc"), localNamed(t, g, "c")
			if !slices.ContainsFunc(c.holderOwners(r), func(o ownerRef) bool { return o.owner == loc }) {
				t.Fatal("capture did not receive the stored borrow before narrowing")
			}
			owners := c.flowOwners(r, flowMemo{})
			if !slices.ContainsFunc(owners, func(o ownerRef) bool { return o.owner == dst }) {
				t.Error("call result lost the reference's actual owner")
			}
			if slices.ContainsFunc(owners, func(o ownerRef) bool { return o.owner == loc }) {
				t.Error("call result kept a borrow stored after the capture")
			}
		})
	}
}
