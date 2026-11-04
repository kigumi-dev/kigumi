package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// irGoldenFixtures are the programs whose LLVM IR is pinned: one plain print, one text
// program, one Map program, one async program and one FFI program. TestIRDump shows the IR
// of every fixture on demand; these five make a structural change prove it kept the IR
// byte-identical instead of only re-running it. UPDATE=1 rewrites the golden files.
var irGoldenFixtures = []string{
	"stdout_write.txtar",
	"text_parse_whitespace.txtar",
	"map_derived_key_move.txtar",
	"async.txtar",
	"f32_native_c_call.txtar",
}

func TestIRGolden(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	out := filepath.Join("testdata", "ir")
	update := os.Getenv("UPDATE") != ""
	for _, name := range irGoldenFixtures {
		t.Run(name, func(t *testing.T) {
			src := runFixtureSource(t, filepath.Join("../../../testdata/run", name))
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"ir\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			ir, ok, err := driver.Compile(m, &stderr)
			if err != nil || !ok {
				t.Fatalf("compile: %v\n%s", err, stderr.String())
			}
			got := ir
			path := filepath.Join(out, strings.TrimSuffix(name, ".txtar")+".ll")
			if update {
				if err := os.MkdirAll(out, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run UPDATE=1 go test ./internal/toolchain/driver -run TestIRGolden)", err)
			}
			if got != string(want) {
				t.Errorf("%s: IR differs from the golden (run with UPDATE=1 to see the diff)", name)
			}
		})
	}
}

// runFixtureSource is the `-- src --` section of a run fixture.
func runFixtureSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	i := strings.Index(text, "-- src --\n")
	if i < 0 {
		t.Fatalf("%s: no -- src -- section", path)
	}
	rest := text[i+len("-- src --\n"):]
	if k := strings.Index(rest, "\n-- "); k >= 0 {
		rest = rest[:k+1]
	}
	return rest
}
