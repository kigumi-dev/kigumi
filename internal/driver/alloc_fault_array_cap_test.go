package driver_test

import (
	"slices"
	"testing"
)

// Each program ends with a push whose Result is dropped, then more pushes: a refusal of the dropped one exits 0,
// so where it falls among the panicking ticks shows whether the capacity the primitive left matches native.
var arrayCapSources = map[string]string{
	"with_capacity": `fn main() {
    let mut xs = Array.tryWithCapacity[Int](2).unwrap()
    xs.push(1)
    xs.push(2)
    let r = xs.tryPush(3)
    for i in 0..12 {
        xs.push(i)
    }
    print "ok"
}
`,
	"insert_at": `fn main() {
    let mut xs = Array.of(1, 2, 3, 4)
    let v = xs.first().unwrap()
    xs.tryInsertAt(0, v).unwrap()
    let r = xs.tryPush(v)
    for i in 0..12 {
        xs.push(v)
    }
    print "ok"
}
`,
	"filter": `fn big(x: &Int) -> Bool {
    x > 1
}

fn main() {
    let a = Array.of(1, 2, 3, 4, 5, 6)
    let mut k = a.tryFilter(big).unwrap()
    k.push(1)
    k.push(2)
    k.push(3)
    let r = k.tryPush(4)
    for i in 0..12 {
        k.push(i)
    }
    print "ok"
}
`,
	"map": `fn big(x: &Int) -> Bool {
    x > 1
}

fn main() {
    let a = Array.of(1, 2, 3)
    let mut k = a.tryMap(big).unwrap()
    let r = k.tryPush(true)
    for i in 0..12 {
        k.push(false)
    }
    print "ok"
}
`,
	"slice": `fn main() {
    let mut k = Array.of(1, 2, 3, 4).trySlice(1, 3).unwrap().unwrap()
    let r = k.tryPush(5)
    for i in 0..12 {
        k.push(i)
    }
    print "ok"
}
`,
	"reserve_room": `fn main() {
    let mut xs = Array.empty[Int]()
    xs.tryReserve(8).unwrap()
    xs.tryPush(1).unwrap()
    xs.tryPush(2).unwrap()
    print "ok"
}
`,
	"append_room": `fn main() {
    let mut xs = Array.of(1, 2)
    xs.tryAppendAll(Array.of(3)).unwrap()
    print "ok"
}
`,
	"concat_push": `fn main() {
    let a = Array.of(1, 2, 3)
    let mut k = a.tryConcat(Array.of(4)).unwrap()
    let r = k.tryPush(1)
    for i in 0..12 {
        k.push(i)
    }
    print "ok"
}
`,
}

// Every primitive that sizes an array leaves the capacity native does, so the push after it takes the same refusable allocation on both backends.
func TestArrayCapacityTicksAgree(t *testing.T) {
	t.Parallel()
	for name, src := range arrayCapSources {
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
