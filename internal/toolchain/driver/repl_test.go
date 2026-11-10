package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func newTestRepl(t *testing.T) (*driver.Repl, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	_, srcFile, _, _ := runtime.Caller(0)
	std := filepath.Join(filepath.Dir(srcFile), "..", "..", "..", "std")
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	var out, errOut bytes.Buffer
	r, err := driver.NewRepl(root, driver.ReplOptions{
		Load: driver.LoadOptions{StdRoot: std, Compiler: testCompilerAt(root)},
		Out:  &out,
		Err:  &errOut,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r, &out, &errOut
}

func TestReplBasicCellSequence(t *testing.T) {
	r, out, errOut := newTestRepl(t)
	res, err := r.Eval("let x = 1", true)
	if err != nil || res.Status != 0 {
		t.Fatalf("let: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval("x + 1", true)
	if err != nil || res.Status != 0 || res.Display != "2" {
		t.Fatalf("x+1: %+v %v\n%s", res, err, errOut.String())
	}
	if out.String() != "" {
		t.Fatalf("driver wrote to stdout unexpectedly: %q", out.String())
	}
}

func TestReplDeclarationsAndFailure(t *testing.T) {
	r, _, errOut := newTestRepl(t)
	for _, src := range []string{
		"fn inc(v: Int) -> Int { v + 1 }",
		"type Point = { pub x Int }",
	} {
		res, err := r.Eval(src, true)
		if err != nil || res.Status != 0 {
			t.Fatalf("declaration %q: %+v %v\n%s", src, res, err, errOut.String())
		}
	}
	res, err := r.Eval("inc(41)", true)
	if err != nil || res.Status != 0 || res.Display != "42" {
		t.Fatalf("fn: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval("Point { x: 3 }.x", true)
	if err != nil || res.Status != 0 || res.Display != "3" {
		t.Fatalf("type: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval("let y = 5", true)
	if err != nil || res.Status != 0 {
		t.Fatalf("let y: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval(`fail "boom"`, true)
	if err != nil || res.Status != 1 || !strings.Contains(res.Message, "boom") {
		t.Fatalf("fail: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval("y", true)
	if err != nil || res.Status != 0 || res.Display != "5" {
		t.Fatalf("after failure: %+v %v\n%s", res, err, errOut.String())
	}
	// Int is Copy: evaluating it does not consume the binding.
	res, err = r.Eval("y", true)
	if err != nil || res.Status != 0 || res.Display != "5" {
		t.Fatalf("copy binding: %+v %v\n%s", res, err, errOut.String())
	}
	for _, src := range []string{
		"type Res = resource { pub id Int }",
		"fn Res.drop(move self) { () }",
		"let z = Res { id: 7 }",
		"z",
	} {
		res, err = r.Eval(src, true)
		if err != nil || res.Status != 0 {
			t.Fatalf("resource %q: %+v %v\n%s", src, res, err, errOut.String())
		}
	}
	res, err = r.Eval("z", true)
	if err != nil || res.Status != 1 || !strings.Contains(res.Message, "z") {
		t.Fatalf("moved binding should be gone: %+v %v\n%s", res, err, errOut.String())
	}
}

func TestReplPlanAndStatus(t *testing.T) {
	r, out, errOut := newTestRepl(t)
	res, err := r.Eval(`$"false"`, true)
	if err != nil || res.Status != 0 || res.Display != "" {
		t.Fatalf("plan: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval(`print "${lastStatus}"`, false)
	if err != nil || res.Status != 0 {
		t.Fatalf("status cell: %+v %v\n%s", res, err, errOut.String())
	}
	if out.String() != "1\n" {
		t.Fatalf("lastStatus stdout = %q", out.String())
	}
}

func TestReplShellOpsAndCompletion(t *testing.T) {
	r, _, errOut := newTestRepl(t)
	dir := t.TempDir()
	res, err := r.Eval(`cd "`+dir+`"`, true)
	if err != nil || res.Status != 0 {
		t.Fatalf("cd: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval(`print pwd()`, false)
	if err != nil || res.Status != 0 {
		t.Fatalf("pwd: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval(`export KIGUMI_REPL_TEST=yes`, true)
	if err != nil || res.Status != 0 {
		t.Fatalf("export: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval(`unset KIGUMI_REPL_TEST`, true)
	if err != nil || res.Status != 0 {
		t.Fatalf("unset: %+v %v\n%s", res, err, errOut.String())
	}
	res, err = r.Eval(`let completeMe = 1`, true)
	if err != nil || res.Status != 0 {
		t.Fatalf("let: %+v %v\n%s", res, err, errOut.String())
	}
	got := r.Completions("comp")
	if len(got) != 1 || got[0] != "completeMe" {
		t.Fatalf("completions = %v", got)
	}
}

func TestReplShadowAndFieldMove(t *testing.T) {
	r, _, errOut := newTestRepl(t)
	if res, err := r.Eval("let x = 1", true); err != nil || res.Status != 0 {
		t.Fatalf("let: %+v %v\n%s", res, err, errOut.String())
	}
	res, err := r.Eval("let x = 2", true)
	if err != nil || res.Status != 1 || !strings.Contains(res.Message, "session binding") {
		t.Fatalf("shadow: %+v %v\n%s", res, err, errOut.String())
	}
	for _, src := range []string{
		"type R = resource { pub v Int }",
		"fn R.drop(move self) { () }",
		"let r = R { v: 9 }",
	} {
		if res, err := r.Eval(src, true); err != nil || res.Status != 0 {
			t.Fatalf("%q: %+v %v\n%s", src, res, err, errOut.String())
		}
	}
	res, err = r.Eval("r.v", true)
	if err != nil || res.Status != 0 || res.Display != "9" {
		t.Fatalf("field read: %+v %v\n%s", res, err, errOut.String())
	}
}
