package printer_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/printer"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func format(src string) string {
	return printer.Print(syntax.Parse(token.NewFile("x.kg", []byte(src))))
}

func TestCommentPlacementIsStable(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"interpolation", "fn f(x: Int) -> String {\n    \"v=${x /* c */}\"\n}\n", ""},
		{"empty if block", "fn g() {\n    if true {\n        // only\n    }\n    print \"a\"\n}\n", ""},
		{"empty else block", "fn g(x: Int) {\n    if x > 0 {\n        print \"a\"\n    } else {\n        // none\n    }\n}\n", ""},
		{"empty block inside interpolation fn", "fn h() {\n    for i in 0..3 {\n        /* c */\n    }\n}\n", ""},
		{"last call trailing", "fn k(x: Int) -> Int {\n    f(x) // last\n}\n", ""},
		{"last match trailing", "fn a(x: Int) -> Int {\n    match x {\n        1 -> 2\n        _ -> 3\n    } // end\n}\n", ""},
		{"last block comment", "fn d(x: Int) {\n    print \"${x}\" /* blk */\n}\n", ""},
		{"own line after last", "fn m(x: Int) -> Int {\n    x\n    // after\n}\n", ""},
	}
	for _, c := range cases {
		once := format(c.src)
		if once != c.src {
			t.Errorf("%s: first run changed canonical input:\n%s", c.name, once)
		}
		if twice := format(once); twice != once {
			t.Errorf("%s: second run differs:\n%s", c.name, twice)
		}
		for _, line := range strings.Split(c.src, "\n") {
			if i := strings.Index(line, "//"); i >= 0 && strings.Count(once, strings.TrimSpace(line[i:])) != 1 {
				t.Errorf("%s: comment %q not kept exactly once", c.name, line[i:])
			}
		}
	}
}

func TestInterpolationCommentIsNotDuplicated(t *testing.T) {
	src := "fn f(x: Int) -> String {\n    \"v=${x /* c */}\"\n}\n"
	out := format(format(format(src)))
	if strings.Count(out, "/* c */") != 1 {
		t.Errorf("comment count changed across runs:\n%s", out)
	}
}

func TestMultiLineStatementKeepsEveryComment(t *testing.T) {
	cases := map[string]string{
		"match arms":    "fn m(x: Int) -> Int {\n    match x {\n        1 -> 2 // one\n        _ -> 3 // two\n    } // end\n}\n",
		"call args":     "fn d(x: Int) -> Int {\n    foo(\n        x, // arg\n    ) // last\n}\n",
		"leading":       "fn d(x: Int) -> Int {\n    let r = foo(\n        // lead\n        x,\n    ) // tail\n}\n",
		"chain":         "fn d(xs: Int) -> Int {\n    let v = xs\n        .map(f) // m1\n        .filter(g) // m2\n}\n",
		"arg then tail": "fn d() -> Int {\n    foo(a, // arg1\n        b) // arg2\n}\n",
		"decorated":     "@derive(Eq) // c3\npub fn f() // c4\n",
	}
	for name, src := range cases {
		once := format(src)
		for _, line := range strings.Split(src, "\n") {
			if i := strings.Index(line, "//"); i >= 0 && strings.Count(once, strings.TrimSpace(line[i:])) != 1 {
				t.Errorf("%s: comment %q lost or duplicated:\n%s", name, line[i:], once)
			}
		}
		if twice := format(once); name != "decorated" && twice != once {
			t.Errorf("%s: second run differs:\n%s", name, twice)
		}
	}
}

func TestEmptyBlockInsideInterpolationKeepsComment(t *testing.T) {
	src := "fn f(x: Int) -> String {\n    \"v=${if x > 0 { /* c */ } else { 1 }}\"\n}\n"
	once := format(src)
	if strings.Count(once, "/* c */") != 1 {
		t.Errorf("comment not kept exactly once:\n%s", once)
	}
	if twice := format(once); twice != once {
		t.Errorf("second run differs:\n%s", twice)
	}
}
