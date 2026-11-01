package driver_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	"kigumi/internal/toolchain/driver"
)

const staticBudgetHead = `interface Touch {
    fn touch(mut self)
}

@impl(Touch)
type Q = { x Int, y Int }
@impl(Touch)
type Pt = { mut x Int, y Int }

fn Q.touch(mut self) {}
fn Pt.touch(mut self) {}
fn mkQ() -> Q { Q { x: 1, y: 2 } }
fn mkPt() -> Pt { Pt { x: 1, y: 2 } }

fn keep[T: Copy + Touch](v: T, n: Int) -> Array[T] {
    let mut out: Array[T] = Array.empty()
    let mut i = 0
    for i < n {
        let mut c = v
        c.touch()
        out.push(c)
        i = i + 1
    }
    out
}

`

// Each kept copy adds an 8-byte array slot; a copy of Pt adds its own 48-byte record (two slots and the header),
// while a copy of Q is native's shared immortal and adds nothing.
func TestStaticCopyMemoryBudget(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, decl, static string
		perCopy            int64
	}{
		{"immortal", "static q: Q = comptime mkQ()", "q", 8},
		{"mut_field", "static origin: Pt = comptime mkPt()", "origin", 8 + 48},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			peak := func(n int) int64 {
				src := staticBudgetHead + c.decl + "\n\nfn main() {\n    let xs = keep(" + c.static + ", " + strconv.Itoa(n) + ")\n    print \"${xs.len()}\"\n}\n"
				a, err := driver.Analyze(loadSweepModule(t, src, ""))
				if err != nil {
					t.Fatal(err)
				}
				if a.Res.HasErrors() {
					t.Fatal(a.Res.Render())
				}
				return minMemoryLimit(t, a.Prog)
			}
			if got := peak(64) - peak(32); got != 32*c.perCopy {
				t.Errorf("32 more kept copies charge %d bytes, want %d", got, 32*c.perCopy)
			}
		})
	}
}

// minMemoryLimit is the smallest sandbox memory limit under which prog runs to completion.
func minMemoryLimit(t *testing.T, prog *mir.Program) int64 {
	t.Helper()
	fits := func(limit int64) bool {
		var errOut bytes.Buffer
		code, err := vm.Run(prog, vm.Options{Sandbox: true, StepLimit: 1 << 40, DepthLimit: 100, MemoryLimit: limit}, &bytes.Buffer{}, &errOut)
		if err != nil && !strings.Contains(err.Error(), "memory limit") {
			t.Fatalf("limit %d: %v", limit, err)
		}
		return err == nil && code == 0
	}
	lo, hi := int64(1), int64(1<<20)
	if !fits(hi) {
		t.Fatalf("does not fit in %d bytes", hi)
	}
	for lo < hi {
		if mid := (lo + hi) / 2; fits(mid) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}
