package driver_test

import (
	"strings"
	"testing"
)

func TestStrictStackCallbackOpsChainIntoCallback(t *testing.T) {
	t.Parallel()
	const prelude = "import alloc from std/alloc\nimport array from std/array\nimport ffi from std/ffi\n\nfn recurse(n: Int) -> Int {\n    if n <= 0 { 0 } else { 1 + recurse(n - 1) }\n}\n\n"
	cases := map[string]string{
		"Box.with":          "fn run() -> Int {\n    let b = alloc.Box.new(0)\n    b.with((v: &Int) => recurse(1000000))\n}\n",
		"Array.tryMap":      "fn run() -> Int {\n    let xs = Array.of(1, 2)\n    let r = xs.tryMap((v: &Int) => recurse(1000000))\n    if r is Ok(_) { 1 } else { 0 }\n}\n",
		"Array.tryFilter":   "fn run() -> Int {\n    let xs = Array.of(1, 2)\n    let r = xs.tryFilter((v: &Int) => recurse(1000000) > 0)\n    if r is Ok(_) { 1 } else { 0 }\n}\n",
		"HandleTable.with":  "fn run() -> Int {\n    let table: ffi.HandleTable[Int] = ffi.HandleTable.new()\n    let h = table.insert(1)\n    let n = table.with(&h, (v: &Int) => recurse(1000000))\n    let _ = table.remove(h)\n    n\n}\n",
		"Array.trySortedBy": "fn run() -> Int {\n    let xs = Array.of(2, 1)\n    let r = array.trySortedBy(&xs, (a: &Int, b: &Int) => if recurse(1000000) > 0 { Ordering.Less } else { Ordering.Greater })\n    if r is Ok(_) { 1 } else { 0 }\n}\n",
	}
	for name, body := range cases {
		out := checkStrict(t, prelude+body+"\nlet total = run()\nprint \"${total}\"\n", true, 0)
		if !strings.Contains(out, "E1410") || !strings.Contains(out, "unbounded") {
			t.Errorf("%s: expected E1410 through the callback, got:\n%s", name, out)
		}
	}
}
