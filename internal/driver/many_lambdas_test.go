package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func TestRunProgramWithManyLambdas(t *testing.T) {
	const n = 1500
	var src strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&src, "let c%d = (x: Int) => x + %d\n", i, i)
	}
	fmt.Fprintf(&src, "print \"${c0(1)} ${c%d(1)}\"\n", n-1)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(src.String()), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("exit %d, err %v\n%s", code, err, errOut.String())
	}
	if want := fmt.Sprintf("1 %d\n", n); out.String() != want {
		t.Errorf("stdout %q, want %q", out.String(), want)
	}
}
