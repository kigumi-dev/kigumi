package driver_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

func TestCoseHeaderValidation32(t *testing.T) {
	requireZig(t)
	for _, target := range []struct{ triple, arch, qemu string }{
		{"x86-linux", "386", "qemu-i386"},
		{"arm-linux", "arm", "qemu-arm"},
	} {
		t.Run(target.triple, func(t *testing.T) {
			runner := target32Runner(t, target.arch, target.qemu)
			root, want := coseHeaderProgram(t)
			config, err := buildconfig.ParseTarget(testCompiler(), target.triple, "")
			if err != nil {
				t.Fatal(err)
			}
			module, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg", Target: config})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "cca32")
			var stderr bytes.Buffer
			if ok, err := driver.BuildWith(module, path, &stderr, driver.BuildOptions{Target: config}); err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, stderr.String())
			}
			argv := append(runner, path)
			out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
			if err != nil || string(out) != want {
				t.Fatalf("run: %v\nstdout=%q\nwant=%q", err, out, want)
			}
		})
	}
}
