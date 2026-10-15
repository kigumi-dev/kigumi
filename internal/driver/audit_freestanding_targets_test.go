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

func largeAggregateProgram(fields int) string {
	var decl, init []string
	for i := range fields {
		decl = append(decl, fmt.Sprintf("    pub f%d Int", i))
		init = append(init, fmt.Sprintf("f%d: %d", i, i))
	}
	return "type Big = {\n" + strings.Join(decl, "\n") + "\n}\n\n" +
		"fn id[T](x: T) -> T {\n    x\n}\n\n" +
		"fn pass[T](x: T) -> Int {\n    let y = id(x)\n    1\n}\n\n" +
		"let b = Big { " + strings.Join(init, ", ") + " }\nprint \"n ${pass(b)}\"\n"
}

// A large aggregate handed to a generic function becomes a memmove call, which the audit must accept on every freestanding target.
func TestBuildFreestandingTargetsPassAudit(t *testing.T) {
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	programs := map[string]string{
		"minimal":         "print \"hi\"\n",
		"large aggregate": largeAggregateProgram(40),
	}
	for _, triple := range []string{"x86_64-freestanding", "x86-freestanding", "arm-freestanding"} {
		for name, src := range programs {
			t.Run(triple+"/"+name, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"audit\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
				os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
				target, err := driver.ParseTarget(driver.CompilerOptions{}, triple, "")
				if err != nil {
					t.Fatal(err)
				}
				std, _ := filepath.Abs("../../std")
				m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Target: target})
				if err != nil {
					t.Fatal(err)
				}
				var buildErr bytes.Buffer
				if ok, err := driver.BuildWith(m, filepath.Join(root, "audit.a"), &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
					t.Fatalf("build: %v\n%s", err, buildErr.String())
				}
			})
		}
	}
}
