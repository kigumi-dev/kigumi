package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

// TestReproducibleIndependentOfCwdAndCache builds the same sources with two
// cwds and two dependency-cache roots and requires one byte-identical output:
// the embedded module and cache paths must not leak into the binary.
func TestReproducibleIndependentOfCwdAndCache(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	write := func(dir, name, body string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(project, "mod.kg", string(manifest("example.com/app", "Source { c: \"x.c\" }\nRequire { name: \"example.com/cdep\", version: \"v1.0.0\" }\n")))
	write(project, "x.c", "int kigumi_probe(void) { return __FILE__[0]; }\n")
	write(project, "main.kg", "import cdep from example.com/cdep\nextern(C) {\n    fn kigumi_probe() -> i32\n    fn kigumi_dep() -> i32\n}\nlet v = unsafe {\n    kigumi_probe() + kigumi_dep()\n}\nprint \"${v} ${cdep.v()}\"\n")
	depFiles := map[string]string{
		"mod.kg":  string(manifest("example.com/cdep", "Source { c: \"c.c\" }\n")),
		"c.c":     "int kigumi_dep(void) { return __FILE__[0]; }\n",
		"cdep.kg": "pub fn v() -> Int {\n    1\n}\n",
	}

	var outputs [][]byte
	for _, cwd := range []string{project, filepath.Join(root, "elsewhere")} {
		for _, cache := range []string{filepath.Join(root, "ca"), filepath.Join(root, "cb", "deep", "er")} {
			compiler := testCompilerAt(cwd)
			compiler.CacheDir = cache
			dir, err := module.CacheDir(compiler, module.Require{Name: "example.com/cdep", Version: "v1.0.0"})
			if err != nil {
				t.Fatal(err)
			}
			for name, body := range depFiles {
				write(dir, name, body)
			}
			write(dir, module.OriginFileName, "cafebabe\n")
			hash, err := module.TreeHash(dir)
			if err != nil {
				t.Fatal(err)
			}
			lock := module.Lock{Entries: map[string]module.LockEntry{
				"example.com/cdep": {Version: "v1.0.0", Commit: "cafebabe", Hash: hash},
			}}
			if err := module.WriteLock(project, lock); err != nil {
				t.Fatal(err)
			}
			m, err := driver.LoadModule(project, driver.LoadOptions{Compiler: compiler, StdRoot: std})
			if err != nil {
				t.Fatalf("load with cwd %s, cache %s: %v", cwd, cache, err)
			}
			if len(m.Sources) != 2 {
				t.Fatalf("expected the module's and the dependency's C sources, got %v", m.Sources)
			}
			out := filepath.Join(t.TempDir(), "prog")
			var stderr bytes.Buffer
			ok, err := driver.BuildWith(m, out, &stderr, driver.BuildOptions{Reproducible: true, Env: os.Environ()})
			if err != nil || !ok {
				t.Fatalf("build with cwd %s, cache %s: %v\n%s", cwd, cache, err, stderr.String())
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			outputs = append(outputs, data)
		}
	}
	for i := 1; i < len(outputs); i++ {
		if off, equal := bytesDiff(outputs[0], outputs[i]); !equal {
			t.Fatalf("build %d differs from build 0 at byte %d (%d vs %d bytes)", i, off, len(outputs[0]), len(outputs[i]))
		}
	}
}
