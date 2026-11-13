package driver_test

import (
	"testing"
	"time"
)

// Left shift discards bits past the width; only the count is checked. Before
// this was pinned, VM treated the discarded bits as an overflow while native
// wrapped, so valid bit-manipulation code panicked only on the VM.
func TestShiftDiscardsOverflowedBits(t *testing.T) {
	const src = `let x: u64 = 0x8000000000000000
let y: u8 = 128
print "${x << 1} ${y << 1} ${x >> 1}"
`
	const want = "0 0 4611686018427387904\n"
	if got := runGeneratedVM(t, src, time.Minute); got != want {
		t.Fatalf("vm: got %q, want %q", got, want)
	}
	if got := runGeneratedAOT(t, src, time.Minute); got != want {
		t.Fatalf("native: got %q, want %q", got, want)
	}
}
