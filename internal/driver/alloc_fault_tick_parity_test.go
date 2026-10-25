package driver_test

import (
	"slices"
	"testing"
)

var tickParitySources = map[string]string{
	"interp": `type E = {}

fn E.tag(self) -> Int {
    1
}

fn mk(n: Int) -> Int {
    let mut c = 0
    for i in 0..n {
        let e = E {}
        c = c + e.tag()
    }
    c
}

fn main() {
    print "${mk(2)}"
}
`,
	"empty_record": `type E = {}

fn E.tag(self) -> Int {
    1
}

fn mk(n: Int) -> Int {
    let mut c = 0
    for i in 0..n {
        let e = E {}
        c = c + e.tag()
    }
    c
}

fn main() {
    let k = mk(2)
    print "ok"
}
`,
	"array_push": `fn main() {
    let mut xs = Array.empty[Int]()
    for i in 0..20 {
        xs.push(i)
    }
    let mut ys = Array.empty[Int]()
    ys.push(1)
    print "ok"
}
`,
	"tryreserve_push": `fn main() {
    let mut xs = Array.empty[Int]()
    let r = xs.tryReserve(8)
    for i in 0..12 {
        xs.push(i)
    }
    print "ok"
}
`,
	"trypush_push": `fn main() {
    let mut xs = Array.empty[Int]()
    let r = xs.tryPush(1)
    for i in 0..12 {
        xs.push(i)
    }
    print "ok"
}
`,
}

// Zero-field records, interpolated strings and Array growth take the same refusable allocations on both backends.
func TestTickParityAgrees(t *testing.T) {
	t.Parallel()
	for name, src := range tickParitySources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := loadSweepModule(t, src, "")
			vmExits := vmSweepExits(t, m)
			if vmExits[0] == 0 {
				t.Fatalf("vm: the first allocation was not refused: %v", vmExits)
			}
			if native := nativeSweepExits(t, m); !slices.Equal(vmExits, native) {
				t.Errorf("fault sweep exits differ\nvm:     %v\nnative: %v", vmExits, native)
			}
		})
	}
}

// The programs these fixtures accept must survive a refusal at every allocation on both backends;
// recoverable_bound_pure_impl is accepted only when the impl that runs is judged, not its declaration.
func TestRecoverableBoundInstanceSurvivesSweep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"recoverable_bound_instance_ok", "recoverable_bound_pure_impl"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := loadSweepModule(t, fixtureSource(t, name), "recoverable-only")
			for backend, exits := range map[string][]int{"vm": vmSweepExits(t, m), "native": nativeSweepExits(t, m)} {
				for i, code := range exits {
					if code != 0 {
						t.Errorf("%s N=%d: exit %d", backend, i+1, code)
					}
				}
			}
		})
	}
}
