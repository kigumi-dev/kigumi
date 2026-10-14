package driver_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// entryCase returns RES from a callee only a boxed caller reaches; MK(v) builds one from v.
type entryCase struct {
	name, decls, setup, call string
}

const entryTemplate = `type Flags = {
    pub a Bool
    pub b Bool
}

fn okFlags(v: Flags) -> Bool { v.a && v.b }

fn okBool(v: Bool) -> Bool { v }

interface Maker {
    fn make(self, i: Int) -> RES
}

type Pm = {
    pub k Int
}

fn Pm.make(self, i: Int) -> RES { MK(i + self.k) }

type Named = {
    pub name String
}

fn Named.make(self, i: Int) -> RES { MK(i) }

fn through(f: fn(Int) -> RES, i: Int) -> RES { f(i) }

%s
fn run(n: Int) -> Int {
    let mut s = 0
    %s
    for i in 0..n {
        if OK(%s) { s = s + 1 }
    }
    s
}

print "ok=${run(%d)}"
`

var entryCases = []entryCase{
	{name: "function value", decls: "fn mk(i: Int) -> RES { MK(i) }\n", call: "through(mk, i)"},
	{name: "method value", call: "through(Pm { k: i }.make, i)"},
	{name: "erased closure", decls: "fn erased(k: Int, i: Int) -> RES {\n    let f: fn(Int) -> RES = (x: Int) => MK(x + k)\n    f(i)\n}\n", call: "erased(1, i)"},
	{name: "method table", decls: "fn viaTable(m: &Maker, i: Int) -> RES { m.make(i) }\n", setup: "let pm: Maker = Pm { k: 1 }", call: "viaTable(&pm, i)"},
	{name: "witness in a shared body", decls: "fn viaWitness[T: Maker](m: &T, i: Int) -> RES { m.make(i) }\n", setup: "let nm = Named { name: \"n\" }", call: "viaWitness(&nm, i)"},
	{name: "bound witness value", decls: "fn viaWitness[T: Maker](m: &T, i: Int) -> RES { m.make(i) }\n", setup: "let nm = Named { name: \"n\" }\n    let g = viaWitness[Named]", call: "g(&nm, i)"},
}

func (c entryCase) src(boxed bool, rounds int) string {
	res, ok, mk := "Bool", "okBool", "(%s > 0)"
	if boxed {
		res, ok, mk = "Flags", "okFlags", "Flags { a: %s > 0, b: true }"
	}
	s := fmt.Sprintf(entryTemplate, c.decls, c.setup, c.call, rounds)
	s = strings.ReplaceAll(s, "RES", res)
	s = strings.ReplaceAll(s, "OK(", ok+"(")
	for {
		i := strings.Index(s, "MK(")
		if i < 0 {
			return s
		}
		depth, j := 1, i+3
		for ; depth > 0; j++ {
			switch s[j] {
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		s = s[:i] + fmt.Sprintf(mk, s[i+3:j-1]) + s[j:]
	}
}

// A Flags of two Bools boxes as a header and a slot array; a Bool is an immortal.
func TestFaultInjectionEntryBoxesCountTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range entryCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			boxed, plain := c.src(true, boxSiteRounds), c.src(false, boxSiteRounds)
			vmBoxes := vmAllocs(t, boxed) - vmAllocs(t, plain)
			nativeBoxes := nativeAllocs(t, boxed) - nativeAllocs(t, plain)
			if vmBoxes != nativeBoxes || vmBoxes < 2*boxSiteRounds {
				t.Errorf("returning Flags through the boxed convention costs %d allocations on the VM and %d on native, want the same and at least %d", vmBoxes, nativeBoxes, 2*boxSiteRounds)
			}
		})
	}
}
