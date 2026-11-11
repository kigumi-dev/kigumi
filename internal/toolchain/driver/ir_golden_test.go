package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// irGoldenFixtures pin pinnedIR of a print, a text, a Map with closures, a Map keyed by a user record,
// an async and an FFI program; UPDATE=1 rewrites testdata/ir.
var irGoldenFixtures = []string{"stdout_write", "text_parse_whitespace", "std_map_update", "conversions", "async", "f32_native_c_call"}

const irGoldenModule = "golden"

func TestIRGolden(t *testing.T) {
	update := os.Getenv("UPDATE") != ""
	for _, name := range irGoldenFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := pinnedIR(compileGolden(t, fixturePath(t, "run/"+name)), irGoldenModule)
			path := filepath.Join("testdata", "ir", name+".ll")
			if update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (UPDATE=1 go test ./internal/toolchain/driver -run TestIRGolden writes it)", err)
			}
			if line, g, w, ok := firstLineDifference(got, string(want)); ok {
				t.Errorf("%s differs at line %d\n got: %s\nwant: %s", path, line, g, w)
			}
		})
	}
}

func compileGolden(t *testing.T, fixture string) string {
	t.Helper()
	fxRoot, _, _, _ := loadRunFixture(t, fixture)
	src, err := os.ReadFile(filepath.Join(fxRoot, "main", "main.kg"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	mod := "Module {\n    name: \"" + irGoldenModule + "\"\n    kigumi: \"0.1\"\n}\n"
	for file, data := range map[string][]byte{"mod.kg": []byte(mod), "main.kg": src} {
		if err := os.WriteFile(filepath.Join(root, file), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tg, err := buildconfig.ParseTarget(testCompiler(), "x86_64-linux", "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: tg})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	return ir
}

func firstLineDifference(got, want string) (int, string, string, bool) {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < max(len(g), len(w)); i++ {
		gl, wl := "<end>", "<end>"
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return i + 1, gl, wl, true
		}
	}
	return 0, "", "", false
}
