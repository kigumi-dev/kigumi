package llgen_test

import (
	"strings"
	"testing"
)

const wrapperSrc = `
interface Shape {
    fn area(self) -> Float
}

type Square = {
    pub side Float
}

type Circle = {
    pub radius Float
}

fn Square.area(self) -> Float {
    self.side * self.side
}

fn Circle.area(self) -> Float {
    self.radius * self.radius
}

fn twice(x: Int) -> Int { x + x }

fn plain(x: Int) -> Int { x + 1 }

fn apply(f: fn(Int) -> Int) -> Int { f(1) }

fn describe[T: Shape](s: T) -> Float { s.area() }

fn main() -> Unit! {
    print "${plain(1)} ${apply(twice)}"
    let s: Shape = Square { side: 2.0 }
    print "${s.area()} ${describe(Square { side: 3.0 })}"
}
`

func defines(ir, sym string) bool {
	return strings.Contains(ir, "define ptr @\""+sym+"\"(")
}

func TestUnreferencedFunctionsHaveNoWrappers(t *testing.T) {
	ir := emitIR(t, wrapperSrc)
	for _, fn := range []string{"main.plain", "main.Circle.area", "main.apply", "main.twice"} {
		if !defines(ir, fn) {
			t.Errorf("%s itself is not emitted", fn)
		}
	}
	for _, sym := range []string{
		"main.plain$adapter", "main.plain$bind", "main.apply$adapter", "main.twice$bind",
		"main.Circle.area$adapter", "main.Circle.area$bind", "main.Circle.area$methodvalue", "main.Square.area$methodvalue",
	} {
		if defines(ir, sym) {
			t.Errorf("%s is defined though nothing refers to it", sym)
		}
	}
}

func TestReferencedWrappersAreDefined(t *testing.T) {
	ir := emitIR(t, wrapperSrc)
	for _, sym := range []string{"main.twice$adapter", "main.Square.area$adapter", "main.Square.area$bind"} {
		if !defines(ir, sym) {
			t.Errorf("%s is referenced but not defined:\n%s", sym, ir)
		}
	}
}

func TestEveryWrapperReferenceIsDefined(t *testing.T) {
	ir := emitIR(t, wrapperSrc)
	refs := 0
	for _, kind := range []string{"$adapter", "$bind", "$methodvalue"} {
		for line := range strings.Lines(ir) {
			i := strings.Index(line, kind+"\"")
			if i < 0 || strings.HasPrefix(line, "define ") {
				continue
			}
			start := strings.LastIndex(line[:i], "@\"")
			sym := line[start+2 : i+len(kind)]
			refs++
			if !defines(ir, sym) {
				t.Errorf("%s is referenced but not defined: %s", sym, strings.TrimSpace(line))
			}
		}
	}
	if refs == 0 {
		t.Error("no wrapper reference found to check")
	}
}
