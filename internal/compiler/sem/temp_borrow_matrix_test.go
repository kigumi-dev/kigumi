package sem_test

import (
	"regexp"
	"strings"
	"testing"
)

const matrixPrelude = `type Res = resource {
    pub id Int
}

fn Res.drop(move self) {}

fn mk(id: Int) -> Res {
    Res { id: id }
}

type Cell = {
    pub v Res
}

fn mkc(id: Int) -> Cell {
    Cell { v: mk(id) }
}

type H['a] = {
    pub mut r &'a Res
}

fn wrap['a](r: &'a Res) -> H['a] {
    H { r: r }
}

type E['a] = Some1(H['a]) | None1

fn look(r: &Res) -> Int {
    r.id
}

context n: Int

@provides(n)
type Num = {
    pub n Int
}

`

type matrixCase struct{ name, code string }

// matrixRoots borrow a value that ends with its statement; keep is the control, a borrow of the caller's cell.
var matrixRoots = []matrixCase{
	{"orCall", "&(om || mk(2))"},
	{"orField", "&(oc || mkc(2)).v"},
	{"matchPayload", "&(match om {\nSome(v) -> v\nNone -> mk(2)\n})"},
	{"ifLetPayload", "&(if let Some(v) = om { v } else { mk(2) })"},
	{"handler", "&(rr || (e) => mk(e))"},
	{"unsafeOr", "&unsafe { om || mk(2) }"},
	{"orUnsafe", "&(om || unsafe {\nlet t = mk(2)\nt\n})"},
	{"deferSplit", "&unsafe {\ndefer {\nif k {\n_ = look(x)\n}\n}\n{\nlet t = mkc(1)\nt.v\n}\n}"},
	{"call", "&mk(1)"},
	{"fieldMove", "&(move c.v)"},
}

const matrixKeep = "&cx.v"

var matrixWraps = []matrixCase{
	{"direct", "R"},
	{"ifThen", "if k { R } else { x }"},
	{"ifElse", "if k { x } else { R }"},
	{"match", "match k {\ntrue -> R\nfalse -> x\n}"},
	{"ifLet", "if let Some(_) = on { R } else { x }"},
	{"nested", "if k {\nmatch k2 {\ntrue -> R\nfalse -> x\n}\n} else {\nx\n}"},
	{"unsafe", "unsafe { R }"},
	{"block", "{\nR\n}"},
	{"context", "context (Num { n: 1 }) { R }"},
}

var matrixPlaces = []matrixCase{
	{"let", "let r = W\nr.id"},
	{"record", "let h = H { r: W }\nh.r.id"},
	{"array", "let xs = Array.of(wrap(W))\nxs[0].r.id"},
	{"call", "let h = wrap(W)\nh.r.id"},
	{"payload", "let e = Some1(wrap(W))\nmatch e {\nSome1(h) -> h.r.id\nNone1 -> 0\n}"},
	{"return", "W"},
	{"store", "let mut h = H { r: x }\nh.r = W\nh.r.id"},
}

// matrixArg hands each borrow to a call that does not keep it: the value outlives the call, so it passes,
// except out of an `if` / `match` arm, whose temporaries end with the arm (E2296).
var matrixArg = matrixCase{"arg", "look(W)"}

var matrixArms = map[string]bool{"ifThen": true, "ifElse": true, "match": true, "ifLet": true, "nested": true}

// matrixTempCodes are the refusals of a borrow kept past the value it names.
var matrixTempCodes = regexp.MustCompile(`error\[(E2234|E2235|E2295|E2296|borrow-escape)\]`)

// matrixKeepGaps are controls the base compiler already refuses with E1040: a `match` of borrows handed to a
// lifetime-tied parameter is not traced to its owners.
var matrixKeepGaps = map[string]bool{"match/array": true, "match/call": true, "match/payload": true}

func matrixProgram(root string, wrap, place matrixCase) string {
	ret := "Int"
	if place.name == "return" {
		ret = "&'a Res"
	}
	body := strings.ReplaceAll(place.code, "W", strings.ReplaceAll(wrap.code, "R", root))
	return matrixPrelude + "fn f['a](x: &'a Res, cx: &'a Cell, c: Cell, om: Res?, oc: Cell?, on: Int?, rr: Result[Res, Int], k: Bool, k2: Bool) -> " +
		ret + " {\n" + body + "\n}\n"
}

// TestTempBorrowMatrix keeps a borrow of a value that ends with its statement in every keeping position,
// through every join wrapper: each is refused, while the caller's cell, or a call that keeps nothing, passes.
func TestTempBorrowMatrix(t *testing.T) {
	check := func(t *testing.T, src string, accept bool) {
		t.Parallel()
		fx := fixture{path: t.Name(), opts: map[string]string{"mir": "planned"}, sections: []section{{name: "src", body: src}}}
		res := checkPlanned(t, fx)
		out := res.Render()
		refused := strings.Contains(out, "error[")
		if accept && refused || !accept && !matrixTempCodes.MatchString(out) {
			t.Errorf("want accepted %v:\n%s\n%s", accept, src, out)
		}
	}
	for _, w := range matrixWraps {
		for _, p := range matrixPlaces {
			t.Run("keep/"+w.name+"/"+p.name, func(t *testing.T) {
				if matrixKeepGaps[w.name+"/"+p.name] {
					t.Skip("E1040 at base")
				}
				check(t, matrixProgram(matrixKeep, w, p), true)
			})
			for _, r := range matrixRoots {
				t.Run(r.name+"/"+w.name+"/"+p.name, func(t *testing.T) { check(t, matrixProgram(r.code, w, p), false) })
			}
		}
		for _, r := range matrixRoots {
			t.Run(r.name+"/"+w.name+"/arg", func(t *testing.T) { check(t, matrixProgram(r.code, w, matrixArg), !matrixArms[w.name]) })
		}
	}
}
