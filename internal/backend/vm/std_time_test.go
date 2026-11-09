package vm

import (
	"math"
	"testing"
	"time"
)

func TestSleepDurationSaturates(t *testing.T) {
	for _, c := range []struct {
		ms   int64
		want time.Duration
	}{
		{0, 0},
		{1500, 1500 * time.Millisecond},
		{math.MaxInt64, math.MaxInt64},
		{18446744073710, math.MaxInt64},
	} {
		if got := sleepDuration(c.ms); got != c.want {
			t.Errorf("sleepDuration(%d) = %d, want %d", c.ms, got, c.want)
		}
	}
}
