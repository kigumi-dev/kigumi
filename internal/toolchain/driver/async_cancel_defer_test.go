package driver_test

import (
	"bytes"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func TestCancelDeferRunsAreDeterministic(t *testing.T) {
	for _, name := range []string{"async_cancel_defer", "async_cancel_defer_stages", "async_cancel_defer_observed", "async_cancel_defer_positions", "async_cancel_defer_panic"} {
		t.Run(name, func(t *testing.T) {
			root, want, _, wantExit := loadRunFixture(t, fixturePath(t, "run/"+name))
			var first string
			for i := 0; i < 2; i++ {
				m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
				if err != nil {
					t.Fatal(err)
				}
				var out, errOut bytes.Buffer
				code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
				if err != nil {
					t.Fatal(err)
				}
				if code != wantExit || out.String() != want {
					t.Fatalf("run %d: exit %d, stdout\n%s\nwant exit %d, stdout\n%s", i, code, out.String(), wantExit, want)
				}
				if i == 0 {
					first = out.String()
				} else if out.String() != first {
					t.Fatalf("run 1 differs from run 0:\n%s\n%s", first, out.String())
				}
			}
		})
	}
}
