package driver_test

import (
	"fmt"
	"math/bits"
	"strings"
)

func addResultCopyOps(ops map[string]tryWrapOp) map[string]tryWrapOp {
	payloads := []struct {
		name, decl, value, typ string
		ticks                  int
		direct                 bool
	}{
		{"cell", "type Cell = { mut n Int }", "Cell { n: 1 }", "Cell", 2, false},
		{"copy_record", "type Point = { n Int }\nfn point(n: Int) -> Point { Point { n: n } }", "point(1)", "Point", 2, false},
		{"option", "type Cell = { mut n Int }", "Some(Cell { n: 1 })", "Cell?", 4, false},
		{"nested_array", "type Cell = { mut n Int }", "FixedArray.filled[Cell, 1](Cell { n: 1 })", "FixedArray[Cell, 1]", 4, false},
		{"scalar", "", "1", "Int", 0, true},
	}
	reads := []struct {
		name, imports, setup, call string
		ticks, scalarTicks         int
		direct                     bool
	}{
		{"get", "", "let xs = Array.of(%[1]s)", "xs.get(0).unwrap()", 2, 0, false},
		{"first", "", "let xs = Array.of(%[1]s)", "xs.first().unwrap()", 2, 0, false},
		{"last", "", "let xs = Array.of(%[1]s)", "xs.last().unwrap()", 2, 0, false},
		{"box_get", "import {Box} from std/alloc", "let xs = Box.new(%[1]s)", "xs.get()", 0, 0, false},
		{"unsafe_cell_get", "import {UnsafeCell} from std/mem", "let xs = UnsafeCell.new(%[1]s)", "unsafe { xs.get() }", 0, 0, false},
		{"uninit_copy", "import {BoundedArray} from std/mem", "let mut xs = BoundedArray.new[%[2]s, 1]()\n    xs.push(%[1]s)", "xs.get(0).unwrap()", 0, 0, false},
		{"error_as", resultCopyError, "let xs: Error = CopyError { payload: %[1]s }", "error.as[CopyError[%[2]s]](&xs).unwrap().payload", 4, 4, false},
		{"fixed_fill", "", "let xs = %[1]s", "FixedArray.filled[%[2]s, 1](xs)", 0, 0, true},
	}
	for _, read := range reads {
		for _, payload := range payloads {
			source := strings.NewReplacer("%[1]s", payload.value, "%[2]s", payload.typ)
			ticks := payload.ticks + read.ticks
			if payload.direct {
				ticks = read.scalarTicks
			}
			if read.direct {
				ticks = 0
			}
			ops["copy_"+read.name+"_"+payload.name] = tryWrapOp{
				imports:   read.imports + "\n" + payload.decl,
				setup:     source.Replace(read.setup),
				call:      source.Replace(read.call),
				panicking: true,
				ticks:     ticks,
			}
		}
	}
	return addBoxedFillOps(ops)
}

func addBoxedFillOps(ops map[string]tryWrapOp) map[string]tryWrapOp {
	for _, record := range []struct{ name, field string }{{"copy_record", "n"}, {"cell", "mut n"}} {
		for _, n := range []int{2, 5} {
			ops[fmt.Sprintf("copy_fixed_fill_boxed_%s_%d", record.name, n)] = tryWrapOp{
				imports:   fmt.Sprintf("type Point = { %s Int }\nfn point(n: Int) -> Point { Point { n: n } }\nfn observe(n: &Int) -> Bool { n == &1 }", record.field),
				setup:     "let xs = point(1)",
				call:      fmt.Sprintf("FixedArray.filled[Point, %d](xs)", n),
				check:     "    if !observe(&r[0].n) {\n        let none: Int? = None\n        none.unwrap()\n    }\n",
				panicking: true,
				ticks:     2*n + bits.Len(uint(n-1)) + 4,
			}
		}
	}
	return ops
}

func resultCopyTickCases() []struct{ name, src string } {
	var cases []struct{ name, src string }
	for name, op := range addResultCopyOps(map[string]tryWrapOp{}) {
		if op.ticks == 0 {
			continue
		}
		cases = append(cases, struct{ name, src string }{name, op.source(false)})
	}
	return cases
}

const resultCopyError = `import error from std/error

type CopyError[T] = { mut payload T }
fn CopyError[T].message(self) -> String { "copy" }
`
