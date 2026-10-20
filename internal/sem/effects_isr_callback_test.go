package sem_test

import (
	"slices"
	"testing"
)

const isrCallbackSrc = `
import {Box} from std/alloc

fn boxWith(b: &Box[Int]) -> Unit {
    b.with((x: &Int) => {})
}

fn boxWithMut(b: &mut Box[Int]) -> Unit {
    b.withMut((x: &mut Int) => {})
}

fn boxWithAlloc(b: &Box[Int]) -> Unit {
    b.with((x: &Int) => {
        _ = "ab".repeat(3)
    })
}

fn boxWithPanic(b: &Box[Int]) -> Unit {
    b.with((x: &Int) => {
        let n: Int? = None
        _ = n.unwrap()
    })
}

fn arrayWithMut(a: &mut Array[Int]) -> Unit {
    _ = a.withMut(0, (x: &mut Int) => {})
}

fn arrayWith(a: &Array[Int]) -> Unit {
    _ = a.with(0, (x: &Int) => {})
}

export(interrupt) {
    pub fn tick() -> Unit {}
}
`

func TestInterruptCallbackPrimitiveSummaries(t *testing.T) {
	res := checkIsr(t, isrCallbackSrc)
	if res.HasErrors() {
		t.Fatalf("source must check:\n%s", res.Render())
	}
	facts := func(name string) ([]string, bool) {
		s := res.OpSummaryOf(lookupFn(t, res, name))
		var out []string
		for _, f := range s.Facts {
			out = append(out, f.Fact)
		}
		return out, s.Untrusted
	}
	for _, name := range []string{"boxWith", "boxWithMut"} {
		if got, untrusted := facts(name); untrusted || len(got) != 0 {
			t.Errorf("%s: an inert Box callback primitive with an inert callback must carry no fact and no distrust: %v untrusted=%v", name, got, untrusted)
		}
	}
	for name, want := range map[string]string{"boxWithAlloc": "allocate", "boxWithPanic": "panic", "arrayWithMut": "allocate", "arrayWith": "allocate"} {
		if got, _ := facts(name); !slices.Contains(got, want) {
			t.Errorf("%s: the callback's and the wrapper's facts must stay visible, want %q in %v", name, want, got)
		}
	}
}
