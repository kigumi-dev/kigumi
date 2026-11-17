package driver_test

import (
	"strings"
	"testing"
)

func TestReplCellStatementOrder(t *testing.T) {
	r, out, _ := newTestRepl(t)
	for _, tc := range []struct {
		src, display, stdout string
		interactive          bool
	}{
		{"let mut x = 1\nx = 4\nprint \"first\"\nprint \"second\"\nx + 1", "5", "first\nsecond\n", true},
		{"x = 7\nlet y = x", "", "", true},
		{"y", "7", "", true},
		{"print \"before\"\nif true { print \"after\" }", "", "before\nafter\n", true},
		{"print \"script\"\nx = 9\nx", "", "script\n", false},
		{"x", "9", "", true},
	} {
		out.Reset()
		res, err := r.Eval(tc.src, tc.interactive)
		if err != nil || res.Status != 0 || res.Display != tc.display || out.String() != tc.stdout {
			t.Fatalf("%q: result=%+v err=%v stdout=%q", tc.src, res, err, out.String())
		}
	}
}

func TestReplLastStatusReserved(t *testing.T) {
	for _, src := range []string{
		`let lastStatus = "hello"`,
		`let (x, lastStatus) = (1, "hello")`,
		`const lastStatus = "hello"`,
		`fn lastStatus() -> String { "hello" }`,
	} {
		t.Run(src, func(t *testing.T) {
			r, _, _ := newTestRepl(t)
			res, err := r.Eval(src, true)
			if err != nil || res.Status != 1 || !strings.Contains(res.Message, "reserved") {
				t.Fatalf("declaration: %+v %v", res, err)
			}
			res, err = r.Eval("lastStatus + 1", true)
			if err != nil || res.Status != 0 || res.Display != "2" {
				t.Fatalf("status after rejection: %+v %v", res, err)
			}
		})
	}
}

func TestReplInitialLastStatus(t *testing.T) {
	r, _, _ := newTestRepl(t)
	res, err := r.Eval("lastStatus", true)
	if err != nil || res.Status != 0 || res.Display != "0" {
		t.Fatalf("initial status: %+v %v", res, err)
	}
}
