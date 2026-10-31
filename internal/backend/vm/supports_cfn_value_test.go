package vm_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

func TestSupportsRefusesForeignFunctionValues(t *testing.T) {
	cases := map[string]string{
		"direct value": `extern(C) {
    fn abs(x: i32) -> i32
}

let g: extern(C) fn(i32) -> i32 = abs
print "before"
print "${unsafe { g(-21) }}"
`,
		"branch value": `extern(C) {
    fn abs(x: i32) -> i32
}

export(C) {
    pub fn twice(x: i32) -> i32 {
        x * 2
    }
}

let flag = false
let g: extern(C) fn(i32) -> i32 = if flag { twice } else { abs }
print "before"
print "${unsafe { g(-21) }}"
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			prog := buildRunnableProgram(t, src)
			ok, why := vm.Supports(prog)
			if ok || !strings.Contains(why, "foreign function abs") {
				t.Fatalf("Supports = %v, %q; want a refusal naming abs", ok, why)
			}
		})
	}
}
