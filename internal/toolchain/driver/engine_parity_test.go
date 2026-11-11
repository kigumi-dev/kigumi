package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// parityFixtures cover one path per opcode family; a divergence between the
// two engines is a bug even when both agree with the golden the fixture
// records, so the comparison is engine to engine and only then to the golden.
var parityFixtures = []string{
	"adt",
	"async",
	"borrow_pattern_field_payload",
	"closures",
	"direct_aggregate",
	"direct_closure",
	"exit_error_handled",
	"fixed_array",
	"float_nan",
	"generics",
	"iface_static",
	"map_debug_insertion_order",
	"static_record_constant",
	"strings_edge",
	"text_parsefloat",
	"tuple",
}

func TestEngineParity(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, name := range parityFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := fixturePath(t, "run/"+name)
			root, wantOut, wantErr, wantExit := loadRunFixture(t, path)
			vmOut, vmErr, vmCode := runFixtureVM(t, root)
			nativeOut, nativeErr, nativeCode := runFixtureNative(t, root)
			if vmOut != nativeOut {
				t.Errorf("stdout differs between engines\n--- vm ---\n%s--- native ---\n%s", vmOut, nativeOut)
			}
			if vmErr != nativeErr {
				t.Errorf("stderr differs between engines\n--- vm ---\n%s--- native ---\n%s", vmErr, nativeErr)
			}
			if vmCode != nativeCode {
				t.Errorf("exit code differs between engines: vm %d, native %d", vmCode, nativeCode)
			}
			if vmOut != wantOut || vmCode != wantExit {
				t.Errorf("engines agree but not with the fixture: got (%q, %d), want (%q, %d)", vmOut, vmCode, wantOut, wantExit)
			}
			if vmErr != wantErr {
				t.Errorf("stderr differs from the fixture\n--- got ---\n%s--- want ---\n%s", vmErr, wantErr)
			}
		})
	}
}

func runFixtureVM(t *testing.T, root string) (string, string, int) {
	t.Helper()
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg", "one", "two"})
	if err != nil {
		t.Fatalf("vm: %v", err)
	}
	return out.String(), errOut.String(), code
}

func runFixtureNative(t *testing.T, root string) (string, string, int) {
	t.Helper()
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe, "one", "two")
	cmd.Dir = t.TempDir()
	cmd.Env = os.Environ()
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	code := 0
	if err := cmd.Run(); err != nil {
		if ee, isExit := err.(*exec.ExitError); isExit {
			code = ee.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return out.String(), errOut.String(), code
}
