package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const dlSrc = "import dl from std/dl\n\nfn run() -> Void! {\n    let lib = host.dl().open(\"libm.so.6\")?\n    _ = lib\n}\n\nrun() || (e) => print \"failed\"\n"

func TestEmitLLVMLinkageMatchesBuild(t *testing.T) {
	zig := testCompiler().CCompiler()
	if len(zig) != 2 || filepath.Base(zig[0]) != "zig" {
		t.Skip("needs zig")
	}
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("needs gcc to stand in for clang")
	}
	fake := t.TempDir()
	if err := os.Symlink(gcc, filepath.Join(fake, "clang")); err != nil {
		t.Fatal(err)
	}
	comp := testCompiler()
	comp.PathDirs = []string{fake}

	cases := []struct {
		name string
		src  string
		link driver.LinkMode
	}{
		{"auto", "print \"hi\"\n", driver.LinkAuto},
		{"dynamic", "print \"hi\"\n", driver.LinkDynamic},
		{"static", "print \"hi\"\n", driver.LinkStatic},
		{"dl static", dlSrc, driver.LinkStatic},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"p\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(root, "main.kg"), []byte(c.src), 0o644)
			std, _ := filepath.Abs("../../../std")
			load := func() *driver.Module {
				m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: comp, StdRoot: std, ZigPath: zig[0]})
				if err != nil {
					t.Fatal(err)
				}
				return m
			}
			var built, emitted driver.BuildReport
			var errBuild, errEmit bytes.Buffer
			_, buildErr := driver.BuildWith(load(), filepath.Join(t.TempDir(), "prog"), &errBuild, driver.BuildOptions{Link: c.link, Report: &built})
			_, _, emitErr := driver.CompileWith(load(), &errEmit, driver.BuildOptions{Link: c.link, Report: &emitted})
			if (buildErr == nil) != (emitErr == nil) || (buildErr != nil && buildErr.Error() != emitErr.Error()) {
				t.Fatalf("build error %v, emit error %v", buildErr, emitErr)
			}
			if buildErr == nil && built.Linkage != emitted.Linkage {
				t.Errorf("build linkage %+v, emit linkage %+v", built.Linkage, emitted.Linkage)
			}
		})
	}
}
