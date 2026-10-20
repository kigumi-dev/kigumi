package llgen_test

import (
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/llgen"
)

func TestStdAdapterReadsBoundValuesInPlace(t *testing.T) {
	ir := specializedIR(t, `let n: Int = 5
let f = n.toFloat
print "${f()}"
`)
	header := regexp.MustCompile(`define ptr @"[^"]*\$stdbound1\$[^"]*"`).FindString(ir)
	if header == "" {
		t.Fatalf("no adapter with one bound value in:\n%s", ir)
	}
	adapter := bodyOf(t, ir, header)
	if !strings.Contains(adapter, "@rt_iter_at(ptr %env, i64 0, i32 1)") {
		t.Errorf("the adapter does not borrow the bound value from the environment:\n%s", adapter)
	}
	if strings.Contains(adapter, "@rt_at(ptr %env") || strings.Contains(adapter, "@rt_release") {
		t.Errorf("the adapter takes a reference to the bound value and must give it back:\n%s", adapter)
	}
}

const stdAdapterOwnedArgSrc = `import ffi from std/ffi

fn callW[F: Fn[fn(&ffi.FfiHandle[Int], fn(&Int) -> Int) -> Int]](w: F, h: &ffi.FfiHandle[Int]) -> Int {
    w(h, (v: &Int) => v + 1)
}

let table: ffi.HandleTable[Int] = ffi.HandleTable.new()
let h = table.insert(1)
print "${callW(table.with, &h)}"
`

func TestStdAdapterRegistersItsOwnedArgumentsForAPanic(t *testing.T) {
	adapterBody := func(ir string) string {
		header := regexp.MustCompile(`define ptr @"[^"]*\$stdbound1\$[^"]*"`).FindString(ir)
		if header == "" {
			t.Fatalf("no adapter with one bound value in:\n%s", ir)
		}
		return bodyOf(t, ir, header)
	}
	p := plannedProgram(t, stdAdapterOwnedArgSrc)
	body := adapterBody(llgen.EmitWith(p, llgen.EmitOptions{Catch: true}))
	enter := "@rt_unwind_enter_std(ptr %urec, ptr "
	at := strings.Index(body, enter)
	if at < 0 {
		t.Fatalf("the adapter registers no record for the callback it retained:\n%s", body)
	}
	if !strings.Contains(body[at:], ", i64 3, i64 3)") {
		t.Errorf("want three arguments with the receiver and the borrowed handle lent:\n%s", body)
	}
	if leave, release := strings.Index(body, "@rt_unwind_leave("), strings.Index(body, "@rt_release("); leave < 0 || release < leave {
		t.Errorf("the record is not left before the retained arguments are released:\n%s", body)
	}
	if body := adapterBody(llgen.EmitWith(plannedProgram(t, stdAdapterOwnedArgSrc), llgen.EmitOptions{})); strings.Contains(body, "@rt_unwind_enter_std(") {
		t.Errorf("a build that catches nothing registers a record:\n%s", body)
	}
}
