package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

func specializedIR(t *testing.T, src string) string {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	passes := []mir.Transform{
		{Name: "plan-rc", Run: mir.PlanRC},
		{Name: "specialize", Run: mir.Specialize},
		{Name: "plan-direct", Run: mir.PlanDirect},
	}
	if err := prog.Apply(passes...); err != nil {
		t.Fatal(err)
	}
	return llgen.EmitProgram(prog, llgen.EmitOptions{Reachable: mir.Reachable(prog)}).IR
}

const identityProbe = `type Point = {
    pub x Int
    pub y Int
}

fn identity[T](x: T) -> T { x }

fn run(n: Int) -> Int {
    let p = identity(Point { x: n, y: n + 1 })
    p.x + p.y
}

print "${run(3)}"
`

func TestIdentityOfARecordNeverBoxes(t *testing.T) {
	ir := specializedIR(t, identityProbe)
	spec := bodyOf(t, ir, `define %"kg.agg.Point" @"main.identity[main.Point]"(%"kg.agg.Point" %p0)`)
	run := bodyOf(t, ir, `define ptr @"main.run"(`)
	for what, body := range map[string]string{"main.identity[main.Point]": spec, "main.run": run} {
		for _, bad := range []string{`@"kg.box.`, `@"kg.unbox.`, "@rt_record("} {
			if strings.Contains(body, bad) {
				t.Errorf("%s calls %s:\n%s", what, bad, body)
			}
		}
	}
	if !strings.Contains(run, `call %"kg.agg.Point" @"main.identity[main.Point]"(%"kg.agg.Point" `) {
		t.Errorf("main.run does not pass the record by value:\n%s", run)
	}
	if strings.Contains(ir, `@"main.identity"(`) {
		t.Error("the shared body is emitted although nothing calls it")
	}
}

func TestWitnessOfARecordIsADirectCall(t *testing.T) {
	ir := specializedIR(t, `interface Measured {
    fn size(self) -> Int
}

type Point = {
    pub x Int
    pub y Int
}

fn Point.size(self) -> Int { self.x + self.y }

fn measure[T: Measured](x: T) -> Int { x.size() * 2 }

print "${measure(Point { x: 1, y: 2 })}"
`)
	body := bodyOf(t, ir, `define ptr @"main.measure[main.Point]"(%"kg.agg.Point" %p0`)
	if !strings.Contains(body, `call ptr @"main.Point.size"(%"kg.agg.Point" `) {
		t.Errorf("size is not called directly:\n%s", body)
	}
	for _, bad := range []string{`@"kg.box.`, "@rt_callv("} {
		if strings.Contains(body, bad) {
			t.Errorf("measure calls %s:\n%s", bad, body)
		}
	}
}

func TestUnreachedInstantiationIsNotEmitted(t *testing.T) {
	ir := specializedIR(t, `type Point = {
    pub x Int
    pub y Int
}

type Other = {
    pub a Int
    pub b Int
}

fn identity[T](x: T) -> T { x }

fn unused(n: Int) -> Int { identity(Other { a: n, b: n }).a }

print "${identity(Point { x: 1, y: 2 }).y}"
`)
	if !strings.Contains(ir, `@"main.identity[main.Point]"(`) {
		t.Error("the reached instantiation is missing")
	}
	if strings.Contains(ir, `@"main.identity[main.Other]"`) {
		t.Error("an instantiation only unused code calls is emitted")
	}
}
