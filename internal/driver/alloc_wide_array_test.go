package driver_test

import (
	"fmt"
	"testing"

	"kigumi/internal/driver"
)

const wideArraySrc = `type Ring = {
    pub slots FixedArray[%[1]s, 4]
    pub n %[1]s
}

fn run(rounds: usize) -> %[1]s {
    let mut r = Ring { slots: FixedArray.filled[%[1]s, 4](1000003), n: 1 }
    let mut xs: Array[%[1]s] = Array.of(2000003)
    for i in 0..rounds {
        let mut s = r.slots
        s[i %% 4] = s[(i + 1) %% 4] + 7000001
        r = Ring { slots: s, n: r.n + 1 }
        xs.push(r.n + 9000001)
    }
    let mut t = r.n
    for x in &xs {
        t = t + x
    }
    t + r.slots[0] + r.slots[1] + r.slots[2] + r.slots[3]
}

print "ok=${run(%[2]d)}"
`

// A 128-bit element is a two-word slot like the narrower widths, so a loop that rebuilds a record holding a fixed array of them costs the same.
func TestWideArrayElementsAllocateLikeNarrowOnes(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	src := func(ty string, rounds int) string { return fmt.Sprintf(wideArraySrc, ty, rounds) }
	perRounds := func(count func(*testing.T, string) int, ty string) int {
		return count(t, src(ty, 8)) - count(t, src(ty, 2))
	}
	for _, wide := range []string{"u128", "i128"} {
		t.Run(wide, func(t *testing.T) {
			t.Parallel()
			narrow := map[string]string{"u128": "u64", "i128": "i64"}[wide]
			w, n := perRounds(nativeAllocs, wide), perRounds(nativeAllocs, narrow)
			t.Logf("native allocations over 6 rounds: %s %d, %s %d", wide, w, narrow, n)
			if w != n {
				t.Errorf("native: %s elements allocate %d over 6 rounds, %s elements %d", wide, w, narrow, n)
			}
			vw, vn := perRounds(vmAllocs, wide), perRounds(vmAllocs, narrow)
			if vw != vn {
				t.Errorf("vm: %s elements allocate %d over 6 rounds, %s elements %d", wide, vw, narrow, vn)
			}
		})
	}
}
