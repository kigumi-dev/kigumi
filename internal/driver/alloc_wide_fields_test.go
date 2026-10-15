package driver_test

import (
	"fmt"
	"testing"

	"kigumi/internal/driver"
)

const wideFieldSrc = `type Acc = {
    pub lo %[1]s
    pub n %[1]s
}

fn run(rounds: Int) -> %[1]s {
    let mut a = Acc { lo: 1, n: 0 }
    let mut p: (%[1]s, %[1]s) = (2, 3)
    for i in 0..rounds {
        a = Acc { lo: a.lo + 3, n: a.n + 1 }
        let (x, y) = p
        p = (x + a.n, y + 1)
    }
    let (x, y) = p
    a.lo + a.n + x + y
}

print "ok=${run(%[2]d)}"
`

// A 128-bit field is a direct slot like the narrower widths, so a loop that rebuilds a record of them costs the same.
func TestWideRecordFieldsAllocateLikeNarrowOnes(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	src := func(ty string, rounds int) string { return fmt.Sprintf(wideFieldSrc, ty, rounds) }
	perRounds := func(count func(*testing.T, string) int, ty string) int {
		return count(t, src(ty, 6)) - count(t, src(ty, 2))
	}
	for _, wide := range []string{"u128", "i128"} {
		t.Run(wide, func(t *testing.T) {
			t.Parallel()
			narrow := map[string]string{"u128": "u64", "i128": "i64"}[wide]
			w, n := perRounds(nativeAllocs, wide), perRounds(nativeAllocs, narrow)
			t.Logf("native allocations over 4 rounds: %s %d, %s %d", wide, w, narrow, n)
			if w != n {
				t.Errorf("native: %s fields allocate %d over 4 rounds, %s fields %d", wide, w, narrow, n)
			}
			if got := perRounds(vmAllocs, wide); got != 0 {
				t.Errorf("vm: %s fields allocate %d over 4 rounds", wide, got)
			}
		})
	}
}
