package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func buildFromCheckout(t *testing.T, checkout string, target driver.Target) []byte {
	t.Helper()
	std, _ := filepath.Abs("../../std")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(checkout, "mod.kg"), []byte("Module {\n    name: \"repro\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(checkout, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	os.WriteFile(filepath.Join(checkout, "extra.c"), []byte("const char *extra_file(void) { return __FILE__; }\nint extra_value(void) { return 7; }\n"), 0o644)
	os.MkdirAll(filepath.Join(checkout, "inc"), 0o755)
	os.WriteFile(filepath.Join(checkout, "inc", "extra.h"), []byte("#define EXTRA 1\n"), 0o644)
	compiler := testCompilerAt(checkout)
	opts := driver.LoadOptions{Compiler: compiler, StdRoot: std, Target: target}
	m, err := driver.LoadModule(checkout, opts)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	var stderr bytes.Buffer
	ok, err := driver.BuildWith(m, out, &stderr, driver.BuildOptions{Target: target, Reproducible: true, Env: os.Environ(), CFlags: []string{"extra.c", "-Iinc"}})
	if err != nil || !ok {
		t.Fatalf("build from %s: %v\n%s", checkout, err, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReproducibleAcrossCheckouts(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	freestanding, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]driver.Target{"hosted": {}, "freestanding": freestanding} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			a := buildFromCheckout(t, filepath.Join(root, "a", "proj"), target)
			b := buildFromCheckout(t, filepath.Join(root, "deeper", "checkout", "proj"), target)
			if off, equal := bytesDiff(a, b); !equal {
				t.Errorf("outputs from two checkouts differ at byte %d (%d vs %d bytes)", off, len(a), len(b))
			}
		})
	}
}

func bytesDiff(a, b []byte) (int, bool) {
	for i := 0; i < min(len(a), len(b)); i++ {
		if a[i] != b[i] {
			return i, false
		}
	}
	return min(len(a), len(b)), len(a) == len(b)
}
