package driver_test

import (
	"testing"
	"time"
)

const fixedFillSource = `const N: usize = 4000000

fn fill() -> FixedArray[Int, N] {
    let mut a = FixedArray.filled[Int, N](0)
    let mut i: usize = 0
    for i < N {
        a[i] = i.toInt() || 0
        i = i + 1
    }
    a
}

let a = fill()
let mut sum = 0
for x in a {
    sum = sum + x
}
print "${sum} ${a[7]}"
`

func TestRunFixedArrayStaysCompact(t *testing.T) {
	out, rss := probedRSS(t, func() string { return runGeneratedVM(t, fixedFillSource, time.Minute) })
	if out != "7999998000000 7\n" {
		t.Fatalf("stdout %q", out)
	}
	t.Logf("peak rss %d MB", rss>>10)
	if rss>>10 > 300 {
		t.Fatalf("peak rss %d MB for 4M scalar elements, limit 300 MB", rss>>10)
	}
}
