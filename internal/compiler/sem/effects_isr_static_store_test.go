package sem_test

import (
	"strings"
	"testing"
)

// Replacing a static mut's heap-owning value releases the old one, so an
// export(interrupt) handler must not do it: the store is the same allocator
// call the handler's own value drops are (E1087/E1080). The four store shapes
// all reach the same drop: the whole static, a field, an element, a helper.
func TestIsrStaticStoreReleasesOldValue(t *testing.T) {
	for name, src := range map[string]string{
		"whole_static": `static mut name: String = "x"

export(interrupt) {
    pub fn ren() {
        unsafe { name = "y" }
    }
}
`,
		"field": `type Slot = { mut name String }
static mut slot: Slot = Slot { name: "x" }

export(interrupt) {
    pub fn ren() {
        unsafe { slot.name = "y" }
    }
}
`,
		"helper": `static mut name: String = "x"

fn replace() {
    unsafe { name = "y" }
}

export(interrupt) {
    pub fn ren() {
        replace()
    }
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			res := checkIsr(t, src)
			out := res.Render()
			if !res.HasErrors() || (!strings.Contains(out, "E1087") && !strings.Contains(out, "E1080")) {
				t.Fatalf("check accepted a handler that frees the value it replaces:\n%s", out)
			}
			if !strings.Contains(out, "drops") && !strings.Contains(out, "deallocate") {
				t.Fatalf("the diagnostic does not name the releasing store:\n%s", out)
			}
		})
	}
}

// The same store outside a handler is ordinary code: the ISR rule is what
// makes it illegal, not the store itself.
func TestNonIsrStaticStoreIsAccepted(t *testing.T) {
	res := checkIsr(t, `static mut name: String = "x"

fn ren() {
    unsafe { name = "y" }
}

fn main() {
    ren()
}
`)
	if res.HasErrors() {
		t.Fatalf("check rejected a plain static store:\n%s", res.Render())
	}
}
