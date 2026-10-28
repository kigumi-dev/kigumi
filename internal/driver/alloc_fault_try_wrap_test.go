package driver_test

import (
	"fmt"
	"slices"
	"testing"
)

const tryWrapSweep = 30

type tryWrapOp struct {
	imports, setup, call string
	check                string
	// fn is the function's own name when it can be held as a fn value and called through it, the shape that keeps the Ok wrap on native.
	fn, args  string
	panicking bool
	ticks     int
}

var tryWrapOps = addResultCopyOps(map[string]tryWrapOp{
	"with_capacity":    {call: "Array.tryWithCapacity[Int](4)", fn: "Array.tryWithCapacity[Int]", args: "4"},
	"bytes_zeros":      {call: "Bytes.tryZeros(4)", fn: "Bytes.tryZeros", args: "4"},
	"byte_buffer_new":  {call: "ByteBuffer.tryNew(4)", fn: "ByteBuffer.tryNew", args: "4"},
	"bytes_from_array": {setup: "let a = Array.of[u8](1, 2)", call: "Bytes.tryFromArray(&a)", fn: "Bytes.tryFromArray", args: "&a"},
	"bytes_concat":     {setup: "let a = Bytes.zeros(2)", call: "a.tryConcat(a)"},
	"bytes_slice":      {setup: "let a = Bytes.zeros(4)", call: "a.trySlice(1, 3)"},
	"string_slice":     {setup: `let s = "hello"`, call: "s.trySliceBytes(1, 3)"},
	"shared_new":       {imports: "import {Shared} from std/alloc", call: "Shared.tryNew(41)", fn: "Shared.tryNew[Int]", args: "41"},
	"shared_get":       {imports: "import {Shared} from std/alloc\n\ntype Cell = {\n    mut n Int\n}", setup: "let s = Shared.new(Cell { n: 1 })", call: "s.tryGet()"},
	"shared_downgrade": {imports: "import {Shared} from std/alloc", setup: "let s = Shared.new(1)", call: "s.tryDowngrade()"},
	"box_new":          {imports: "import {Box} from std/alloc", call: "Box.tryNew(41)", fn: "Box.tryNew[Int]", args: "41"},
	"map_clone":        {imports: "import map from std/map", setup: "let mut m = map.empty[Int, Int]()\n    m.set(1, 10)", call: "m.tryClone()"},
	"map_try_empty":    {imports: "import map from std/map", call: "map.tryEmpty[Int, Int]()", fn: "map.tryEmpty[Int, Int]"},
	"builder_new":      {imports: "import text from std/text", call: "text.StringBuilder.tryNew()", fn: "text.StringBuilder.tryNew"},
	"os_string_clone":  {imports: "import os from std/os", setup: "let s = os.OsString.fromUnixBytes(Bytes.zeros(3))", call: "s.tryClone()"},
})

func (o tryWrapOp) source(viaValue bool) string {
	setup, call := "    "+o.setup+"\n", o.call
	if viaValue {
		setup += "    let f = " + o.fn + "\n"
		call = "f(" + o.args + ")"
	}
	check := "    if r is Err(_) {\n        let none: Int? = None\n        none.unwrap()\n    }\n"
	if o.panicking {
		check = o.check
	}
	return fmt.Sprintf("%s\n\nfn main() {\n%s    eprint \"go\"\n    let r = %s\n%s    print \"ok\"\n}\n", o.imports, setup, call, check)
}

// A try op takes the refusable allocations native's runtime does, wrap included, whichever way it is called.
func TestTryWrapTicksAgree(t *testing.T) {
	t.Parallel()
	for name, op := range tryWrapOps {
		for _, viaValue := range []bool{false, true} {
			if viaValue && op.fn == "" {
				continue
			}
			t.Run(fmt.Sprintf("%s/value=%v", name, viaValue), func(t *testing.T) {
				t.Parallel()
				m := loadSweepModule(t, op.source(viaValue), "")
				vmExits := vmSweepExitsTo(t, m, tryWrapSweep, true)
				if vmExits[0] == 0 && (!op.panicking || op.ticks > 0) {
					t.Fatalf("vm: the first allocation was not refused: %v", vmExits)
				}
				if op.panicking {
					for i, exit := range vmExits {
						want := 0
						if i < op.ticks {
							want = 2
						}
						if exit != want {
							t.Errorf("vm N=%d: exit %d, want %d", i+1, exit, want)
						}
					}
				}
				if native := nativeSweepExitsTo(t, m, tryWrapSweep, true); !slices.Equal(vmExits, native) {
					t.Errorf("fault sweep exits differ\nvm:     %v\nnative: %v", vmExits, native)
				}
			})
		}
	}
}

var tryWrapPrograms = map[string]string{
	"map_clone_set": `import map from std/map

fn main() {
    let mut m = map.empty[Int, Int]()
    m.set(1, 10)
    m.set(2, 20)
    print "A"
    let r = m.tryClone()
    print "B"
    let mut c = r.unwrap()
    print "C"
    c.set(3, 30)
    print "D"
    c.set(4, 40)
    print "E"
}
`,
	"slice_cells": `type Cell = {
    mut n Int
}

fn main() {
    let a = Array.of(Cell { n: 1 }, Cell { n: 2 })
    print "A"
    let r = a.trySlice(0, 2)
    print "B"
    let b = r.unwrap().unwrap()
    print "C ${b.len()}"
}
`,
	"map_try_empty": `import map from std/map

fn main() {
    print "A"
    let r = map.tryEmpty[Int, Int]()
    print "B"
    let mut m = r.unwrap()
    print "C"
}
`,
	"builder_try_new": `import text from std/text

fn main() {
    print "A"
    let r = text.StringBuilder.tryNew()
    print "B"
    let mut b = r.unwrap()
    print "C"
}
`,
	"eprint_resets_only_when_armed": `fn main() {
    let mut xs = Array.empty[Int]()
    eprint "a"
    xs.tryPush(1).unwrap()
    eprint "b"
    xs.push(2)
    eprint "c"
    print "ok"
}
`,
}

// The programs that exposed the divergences run unarmed, ticks before the first eprint included.
func TestTryWrapProgramsAgree(t *testing.T) {
	t.Parallel()
	for name, src := range tryWrapPrograms {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := loadSweepModule(t, src, "")
			vmExits := vmSweepExitsTo(t, m, tryWrapSweep, false)
			if vmExits[0] == 0 {
				t.Fatalf("vm: the first allocation was not refused: %v", vmExits)
			}
			if native := nativeSweepExitsTo(t, m, tryWrapSweep, false); !slices.Equal(vmExits, native) {
				t.Errorf("fault sweep exits differ\nvm:     %v\nnative: %v", vmExits, native)
			}
		})
	}
}
