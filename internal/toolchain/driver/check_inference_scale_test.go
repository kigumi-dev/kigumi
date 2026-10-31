package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func TestCheckOnlyDeepGenericInference(t *testing.T) {
	out, _, cpu := probedUsage(t, func() string {
		const depth = 2400
		src := "fn wrap[T](x: T) -> Option[T] { Some(x) }\nfn work() -> Int {\nlet v = " + strings.Repeat("wrap(", depth) + "1" + strings.Repeat(")", depth) + "\n0\n}\nprint \"${work()}\"\n"
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
		if err != nil {
			t.Fatal(err)
		}
		r, err := driver.CheckOnly(m)
		if err != nil {
			t.Fatal(err)
		}
		if r.HasErrors() {
			t.Fatal(r.Render())
		}
		return "checked"
	})
	if out != "checked" {
		t.Fatalf("output = %q", out)
	}
	t.Logf("check used %v of CPU time", cpu)
	if cpu > 4*time.Second {
		t.Fatalf("cpu time %v, limit 4s", cpu)
	}
}
