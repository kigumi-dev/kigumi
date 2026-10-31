package driver_test

import (
	"fmt"
	"testing"
)

const resultMutTemplate = `type In = {
    pub mut a Int
    pub mut b Int
}

fn make(i: Int) -> Result[%[1]s, Int] {
    if i %% 3 == 2 { Err(i) } else { Ok(%[2]s) }
}

fn step(i: Int) -> Result[Int, Int] {
    let mut x = make(i)?
    %[3]s
    Ok(%[4]s)
}

fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        match step(i) {
            Ok(v) -> { s = s + v }
            Err(e) -> { s = s - e }
        }
    }
    s
}

print "ok=${run(%[5]d)}"
`

var resultMutCases = []struct{ name, typ, val, write, read string }{
	{"record", "In", "In { a: i, b: 1 }", "x.a = x.a + 5", "x.a + x.b"},
	{"fixed array", "FixedArray[Int, 3]", "FixedArray.filled[Int, 3](i)", "x[1] = x[1] + 5", "x[0] + x[1]"},
}

func TestFaultInjectionMutableUnwrappedResultPayloadsAllocateNothingPerRound(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range resultMutCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := func(n int) string { return fmt.Sprintf(resultMutTemplate, c.typ, c.val, c.write, c.read, n) }
			vmPer := vmAllocs(t, src(12)) - vmAllocs(t, src(4))
			nativePer := nativeAllocs(t, src(12)) - nativeAllocs(t, src(4))
			if vmPer != 0 || nativePer != 0 {
				t.Errorf("8 more rounds cost %d allocations on the VM and %d on native, want none", vmPer, nativePer)
			}
		})
	}
}
