package llgen_test

import (
	"regexp"
	"strings"
	"testing"
)

const definedOverAdtSrc = `
type Tick = Up(n: Int) | Down(n: Int) | Flat
type Marked = Tick

fn Tick.value(self) -> Int {
    match self {
        Up(n) -> n
        Down(n) -> 0 - n
        Flat -> 0
    }
}

fn bumpTo(t: &mut Tick, n: Int) { t = Up(n) }

fn marked(n: Int) -> Int {
    let mut m = Marked(Up(n * 2))
    bumpTo(&mut m.value, n + 1)
    m.value.value()
}

fn main() -> Void! { print "${marked(3)}" }
`

// An alias renames the crossing's box instead of consuming it, so releasing at the alias would free the binding's object.
func TestDirectAdtCrossingAtAnAliasIsReleasedByItsBinding(t *testing.T) {
	body := bodyOf(t, emitIR(t, definedOverAdtSrc), `@"main.marked"(`)
	box := regexp.MustCompile(`(%v\d+) = call ptr @"kg\.box\.main\.Tick"\(`).FindStringSubmatch(body)
	if box == nil {
		t.Fatalf("marked never crosses into the boxed form:\n%s", body)
	}
	slot := regexp.MustCompile(`store ptr ` + box[1] + `, ptr (%l\d+)\n`).FindStringSubmatch(body)
	if slot == nil {
		t.Fatalf("the box %s reaches no slot:\n%s", box[1], body)
	}
	loads := regexp.MustCompile(`(%v\d+) = load ptr, ptr `+slot[1]+`\n`).FindAllStringSubmatch(body, -1)
	if len(loads) == 0 {
		t.Fatalf("nothing reads %s back:\n%s", slot[1], body)
	}
	for _, l := range loads {
		if strings.Contains(body, "call void @rt_release(ptr "+l[1]+")") {
			t.Errorf("the crossing's box is released at the alias (%s):\n%s", l[1], body)
		}
	}
	if !strings.Contains(body, "@rt_drop(") {
		t.Errorf("nothing drops the binding the box moved into:\n%s", body)
	}
}

const fixedCrossIRSrc = `
fn pick[T](v: T) -> T { v }

fn fixed() -> Bool { pick(Some(4)) is Some(_) }

fn fresh(n: Int) -> Bool { pick(Some(n)) is Some(_) }

fn main() -> Void! { print "${fixed()} ${fresh(1)}" }
`

// A fixed crossing reads the variant's immortal and calls no boxing helper; the second builds its box at run time.
func TestFixedCrossingReadsTheImmortal(t *testing.T) {
	ir := emitIR(t, fixedCrossIRSrc)
	fixed := bodyOf(t, ir, `@"main.fixed"(`)
	if strings.Contains(fixed, `@"kg.box.`) || !strings.Contains(fixed, `@".constval.`) {
		t.Errorf("a fixed crossing does not read the immortal:\n%s", fixed)
	}
	if fresh := bodyOf(t, ir, `@"main.fresh"(`); !strings.Contains(fresh, `call ptr @"kg.box.Option[Int]"(`) {
		t.Errorf("a crossing built at run time does not box:\n%s", fresh)
	}
}
