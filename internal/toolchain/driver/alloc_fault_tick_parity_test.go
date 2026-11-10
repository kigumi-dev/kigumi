package driver_test

import (
	"slices"
	"testing"
)

var tickParitySources = map[string]string{
	"share_copy_fill": `type Text = { s String }
fn text(s: String) -> Text { Text { s: s } }
fn main() {
    let value = text("hello")
    eprint "go"
    let xs = FixedArray.filled[Text, 1](value)
    if xs[0].s != "hello" { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"share_copy_duplicate": `type Text = { s String }
fn text(s: String) -> Text { Text { s: s } }
fn duplicate[T: Copy](value: T) -> (T, T) { (value, value) }
fn main() {
    let value = text("hello")
    eprint "go"
    let (left, right) = duplicate(value)
    if left.s != "hello" || right.s != "hello" { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"share_copy_mutable": `type Text = { mut s String }
fn text(s: String) -> Text { Text { s: s } }
fn duplicate[T: Copy](value: T) -> (T, T) { (value, value) }
fn main() {
    let value = text("hello")
    eprint "go"
    let (first, right) = duplicate(value)
    let mut left = first
    left.s = "changed"
    if right.s != "hello" { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
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
    xs.tryReserve(8).unwrap()
    for i in 0..12 {
        xs.push(i)
    }
    print "ok"
}
`,
	"trypush_push": `fn main() {
    let mut xs = Array.empty[Int]()
    xs.tryPush(1).unwrap()
    for i in 0..12 {
        xs.push(i)
    }
    print "ok"
}
`,
}

// An `allocator` block's guard and an async Future are runtime headers native
// builds with opaque_in/rt_opaque — one refusable allocation each. Neither op
// has an Err form, so the refusal is fatal on both backends, unlike a try*
// primitive whose refusal is reported. The eprint arms the sweep in both, so
// only the guarded window is counted.
var opaqueHeaderSources = map[string]string{
	"allocator_block_scope": `import {Arena} from std/alloc
fn main() -> Void! {
    let a = Arena.create()?
    eprint "go"
    let n = allocator a.scope() { Bytes.tryZeros(16).unwrap().len() }
    print "ok ${n}"
}
`,
	"async_future_run": `import task from std/task
async fn f() -> Result[Int, AllocError] {
    let b = Bytes.tryZeros(4)?
    Ok(b.len().toInt() || 0)
}
fn main() {
    eprint "go"
    let ex = task.currentThreadExecutor()
    match ex.runToCompletion(f()) { Ok(n) -> print "ok ${n}", Err(_) -> print "err" }
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

// Both headers are fatal when refused, so the sweep stays outside every try: the VM must refuse at the same N as native.
func TestOpaqueHeaderTicksAgree(t *testing.T) {
	t.Parallel()
	for name, src := range opaqueHeaderSources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := loadSweepModule(t, src, "")
			vmExits := vmSweepExitsTo(t, m, bindSweep, true)
			if vmExits[0] == 0 {
				t.Fatalf("vm: the first allocation after eprint was not refused: %v", vmExits)
			}
			if native := nativeSweepExitsTo(t, m, bindSweep, true); !slices.Equal(vmExits, native) {
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
