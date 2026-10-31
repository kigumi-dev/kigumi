package llgen_test

import (
	"regexp"
	"strings"
	"testing"
)

const stdValueProbe = `import ffi from std/ffi

let make = ffi.CString.new
let measure = ffi.sizeOf[i64]
let n: i64 = 1
print "${measure(&n)}"
match make(&"hi") {
    Ok(_) -> print "ok"
    Err(_) -> print "err"
}
`

func TestStdFunctionValueAdaptersOwnTheirStackSlots(t *testing.T) {
	ir := specializedIR(t, stdValueProbe)
	adapter := bodyOf(t, ir, `define ptr @"std/ffi.CString.new$stdbound0$`)
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`(%v\d+) = `).FindAllStringSubmatch(adapter, -1) {
		defined[m[1]] = true
	}
	if !strings.Contains(adapter, "= alloca") {
		t.Errorf("the adapter reserves no argument array of its own:\n%s", adapter)
	}
	for _, m := range regexp.MustCompile(`%v\d+`).FindAllString(adapter, -1) {
		if !defined[m] {
			t.Errorf("the adapter uses %s, which only another function defines:\n%s", m, adapter)
		}
	}
}

func TestGenericStdFunctionValueIsExpandedPerInstantiation(t *testing.T) {
	ir := specializedIR(t, stdValueProbe)
	if !strings.Contains(ir, `define ptr @"std/ffi.sizeOf$stdbound0$pure fn(&Int)->usize`) {
		t.Errorf("sizeOf[i64] as a value has no adapter of its own instantiation:\n%s", ir)
	}
}

func TestMethodValueAdapterBorrowsALentReceiver(t *testing.T) {
	ir := specializedIR(t, `type P = {
    a Int
}

fn P.get(self) -> Int { self.a }

let p = P { a: 1 }
let g = p.get
print "${g()}"
`)
	adapter := bodyOf(t, ir, `define ptr @"main.P.get$methodvalue"`)
	borrowed := strings.Contains(adapter, "@rt_iter_at(ptr %env, i64 0, i32 1)")
	if !borrowed && !strings.Contains(adapter, "@rt_release") {
		t.Errorf("the adapter never drops the receiver reference it took from the environment:\n%s", adapter)
	}
	if borrowed && strings.Contains(adapter, "@rt_release") {
		t.Errorf("the adapter releases a receiver it only borrowed:\n%s", adapter)
	}
}
