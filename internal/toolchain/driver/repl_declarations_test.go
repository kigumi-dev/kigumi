package driver_test

import (
	"strings"
	"testing"
)

func TestReplRejectsDeclarationThatFailsLowering(t *testing.T) {
	r, _, _ := newTestRepl(t)
	res, err := r.Eval("fn bad() -> usize { let mut a = Array.of(1); let r = &a; a.push(2); r.len() }", true)
	if err != nil || res.Status != 1 || !strings.Contains(res.Message, "E2198") {
		t.Fatalf("declaration: %+v %v", res, err)
	}
	for _, tc := range []struct{ src, display string }{
		{"1 + 1", "2"},
		{"lastStatus", "0"},
	} {
		res, err = r.Eval(tc.src, true)
		if err != nil || res.Status != 0 || res.Display != tc.display {
			t.Fatalf("%q after rejection: %+v %v", tc.src, res, err)
		}
	}
	if res, err = r.Eval("bad()", true); err != nil || res.Status != 1 || res.Message == "" {
		t.Fatalf("calling the rejected declaration: %+v %v", res, err)
	}
}

func TestReplDuplicateBindingSetsStatus(t *testing.T) {
	r, _, _ := newTestRepl(t)
	for _, src := range []string{"let x = 1", "let x = 2"} {
		if _, err := r.Eval(src, true); err != nil {
			t.Fatal(err)
		}
	}
	res, err := r.Eval("lastStatus", true)
	if err != nil || res.Display != "1" {
		t.Fatalf("status after duplicate binding: %+v %v", res, err)
	}
}

func TestReplUserGenericAcrossCells(t *testing.T) {
	r, _, _ := newTestRepl(t)
	for _, tc := range []struct{ src, display string }{
		{"fn id[T](x: T) -> T { x }", ""},
		{"id(1)", "1"},
		{`let v = id("a")`, ""},
		{"v", "a"},
		{"id(2)", "2"},
		{"id(1.5)", "1.5"},
	} {
		res, err := r.Eval(tc.src, true)
		if err != nil || res.Status != 0 || res.Display != tc.display {
			t.Fatalf("%q: %+v %v", tc.src, res, err)
		}
	}
}

func TestReplDeclarationDiagnosticLine(t *testing.T) {
	r, _, _ := newTestRepl(t)
	if res, err := r.Eval("fn ok() -> Int { 1 }", true); err != nil || res.Status != 0 {
		t.Fatalf("ok: %+v %v", res, err)
	}
	res, err := r.Eval("fn bad() -> usize { let mut a = Array.of(1); let r = &a; a.push(2); r.len() }", true)
	if err != nil || res.Status != 1 || !strings.Contains(res.Message, "cell.kg:1:") {
		t.Fatalf("lowering diagnostic: %+v %v", res, err)
	}
}

func TestReplUserFnNamedValidate(t *testing.T) {
	r, _, _ := newTestRepl(t)
	if res, err := r.Eval("fn __repl_validate() -> Int { 1 }", true); err != nil || res.Status != 0 {
		t.Fatalf("declaration: %+v %v", res, err)
	}
	res, err := r.Eval("__repl_validate()", true)
	if err != nil || res.Status != 0 || res.Display != "1" {
		t.Fatalf("call: %+v %v", res, err)
	}
}
