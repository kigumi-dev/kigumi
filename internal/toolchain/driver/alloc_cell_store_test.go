package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func closureNoalloc(t *testing.T, src, owner string) bool {
	t.Helper()
	a, err := driver.Analyze(editorModule(t, src))
	if err != nil || a.Prog == nil {
		t.Fatalf("analyze: %v", err)
	}
	for _, f := range a.Prog.Funcs {
		if strings.Contains(f.Name, "."+owner+".closure") {
			return a.Res.Summary(f.Ent).Noalloc
		}
	}
	t.Fatalf("no closure under %s", owner)
	return false
}

func TestClosureStoringADirectValueIntoACellMayAllocate(t *testing.T) {
	t.Parallel()
	const head = `type Point = {
    pub x Int
    pub y Int
}

fn find(n: Int) -> u8? { n.toU8() }

fn mk(n: Int) -> Point { Point { x: n, y: n } }
`
	cases := map[string]string{
		"option": "fn run(n: Int) -> Int {\n    let mut c: u8? = None\n    let f = () => { c = find(n) }\n    f()\n    (c || 0).toInt()\n}\n",
		"record": "fn run(n: Int) -> Int {\n    let mut c = Point { x: 0, y: 0 }\n    let f = () => { c = mk(n) }\n    f()\n    c.x\n}\n",
	}
	for name, body := range cases {
		if closureNoalloc(t, head+body+"\nfn main() -> Void! {}\n", "run") {
			t.Errorf("%s: the closure boxes the value it stores, so its summary must not claim noalloc", name)
		}
	}
}

func TestClosureStoringAScalarIntoACellStaysNoalloc(t *testing.T) {
	t.Parallel()
	src := "fn run(n: Int) -> Int {\n    let mut c = 0\n    let f = () => { c = n }\n    f()\n    c\n}\n\nfn main() -> Void! {}\n"
	if !closureNoalloc(t, src, "run") {
		t.Error("a scalar store into a cell does not allocate")
	}
}
