package driver_test

import (
	"fmt"
	"testing"
)

type resultCase struct {
	name, decls, ok, err, use string
}

const resultTemplate = `%s
fn make(i: Int) -> Result[%s, %s] {
    if i %% 3 == 2 { Err(%s) } else { Ok(%s) }
}

fn step(i: Int) -> Result[%s, %s] {
    let v = make(i)?
    Ok(v)
}

fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        match step(i) {
            Ok(v) -> { s = s + %s }
            Err(e) -> { s = s + %s }
        }
    }
    s
}

print "ok=${run(%d)}"
`

var resultCases = []resultCase{
	{name: "int and int", ok: "Int", err: "Int", use: "v|e"},
	{name: "record payload", decls: "type Point = {\n    pub x Int\n    pub y Int\n}\n", ok: "Point", err: "Int", use: "v.x + v.y|e"},
	{name: "variant error", decls: "type Fault = Bad(n: Int) | Worse\n", ok: "Int", err: "Fault", use: "v|(match e {\n                Bad(n) -> n\n                Worse -> 0\n            })"},
	{name: "unit payload", ok: "Void", err: "Int", use: "1|e"},
	{name: "bool and float", ok: "Bool", err: "Float", use: "(if v { 1 } else { 0 })|(if e > 1.0 { 2 } else { 3 })"},
}

func (c resultCase) src(rounds int) string {
	mk := map[string][2]string{
		"Int": {"i", "i"}, "Point": {"Point { x: i, y: 1 }", ""}, "Fault": {"", "Bad(i)"},
		"Void": {"()", ""}, "Bool": {"i > 0", ""}, "Float": {"", "1.5"},
	}
	okv, errv := mk[c.ok][0], mk[c.err][1]
	if c.ok == "Int" {
		okv = "i"
	}
	if c.err == "Int" {
		errv = "i"
	}
	var okUse, errUse string
	for i, part := range splitUse(c.use) {
		if i == 0 {
			okUse = part
		} else {
			errUse = part
		}
	}
	return fmt.Sprintf(resultTemplate, c.decls, c.ok, c.err, errv, okv, c.ok, c.err, okUse, errUse, rounds)
}

func splitUse(s string) []string {
	depth := 0
	for i, r := range s {
		switch r {
		case '(', '{':
			depth++
		case ')', '}':
			depth--
		case '|':
			if depth == 0 {
				return []string{s[:i], s[i+1:]}
			}
		}
	}
	return []string{s}
}

func TestFaultInjectionDirectResultLoopsAllocateNothingPerRound(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range resultCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			short, long := c.src(4), c.src(12)
			vmPer := vmAllocs(t, long) - vmAllocs(t, short)
			nativePer := nativeAllocs(t, long) - nativeAllocs(t, short)
			if vmPer != 0 || nativePer != 0 {
				t.Errorf("8 more rounds cost %d allocations on the VM and %d on native, want none", vmPer, nativePer)
			}
		})
	}
}

const fixedProtocolTemplate = `type Cell = {
    pub a Int
    pub b Bool
}

type Grid = {
    pub tag u8
    pub cells FixedArray[Cell, 3]
}

type Seen = Hit(g: Grid) | Miss

fn make(i: Int) -> Grid {
    let mut c = FixedArray.filled[Cell, 3](Cell { a: 0, b: false })
    c[1] = Cell { a: i, b: true }
    Grid { tag: 9, cells: c }
}

fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        let g = make(i %% 3)
        let h = make(1)
        if g == h { s = s + 1 }
        if hash(&g) == hash(&h) { s = s + 2 }
        if Hit(g) == Hit(h) { s = s + 4 }
    }
    s
}

print "ok=${run(%d)}"
`

func TestFaultInjectionFixedArrayProtocolsAllocateNothingPerRound(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	short, long := fmt.Sprintf(fixedProtocolTemplate, 4), fmt.Sprintf(fixedProtocolTemplate, 12)
	vmPer := vmAllocs(t, long) - vmAllocs(t, short)
	nativePer := nativeAllocs(t, long) - nativeAllocs(t, short)
	if vmPer != 0 || nativePer != 0 {
		t.Errorf("8 more rounds cost %d allocations on the VM and %d on native, want none", vmPer, nativePer)
	}
}
