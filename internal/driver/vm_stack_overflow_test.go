package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

const stackOverflowTestSrc = `import test from std/test

fn deep(n: Int) -> Int {
    1 + deep(n + 1)
}

test "runaway recursion fails" {
    let _ = deep(0)
}

test "the next test still runs" {
    test.expectEq(1 + 1, 2)?
}
`

// A runaway recursion must fail its own test like any panic, not take the runner down.
func TestStackOverflowFailsOnlyItsTest(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main/main.kg"), []byte(stackOverflowTestSrc), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Test: true, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunTestsWith(m, driver.RunOptions{}, &out, &errOut)
	if err != nil {
		t.Fatalf("%v\n%s%s", err, out.String(), errOut.String())
	}
	if code == 0 {
		t.Fatalf("want a failed test, got exit 0\n%s%s", out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `FAIL "runaway recursion fails"`) || !strings.Contains(out.String(), `ok   "the next test still runs"`) {
		t.Errorf("stdout = %q", out.String())
	}
	if !strings.Contains(errOut.String(), "stack overflow in main.deep") {
		t.Errorf("stderr = %q, want the stack overflow panic naming the function", errOut.String())
	}
}
