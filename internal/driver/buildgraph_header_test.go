package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func TestBuildProgramHeaderWithAddedSource(t *testing.T) {
	t.Parallel()
	needLinuxZig(t)
	root := t.TempDir()
	write := func(rel, body string) {
		os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755)
		os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644)
	}
	write("mod.kg", "Module {\n    name: \"hdrsrc\"\n    kigumi: \"0.1\"\n}\n\nHeader {\n    path: \"libc.h\"\n    package: \"form\"\n}\n")
	write("libc.h", "#include <stdio.h>\n")
	write("extra.c", "int extra_value(void) { return 42; }\n")
	write("build.kg", "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let app = b.executable(\"app\", \"cmd/app\", b.request().target())?\n    app.addSource(b.file(\"extra.c\")?)\n}\n")
	write("cmd/app/main.kg", "import form from c/form\n\nextern(C) {\n    fn extra_value() -> i32\n}\n\nprint \"${form.BUFSIZ} ${unsafe { extra_value() }}\"\n")
	std, _ := filepath.Abs("../../std")
	for _, link := range []driver.LinkMode{driver.LinkAuto, driver.LinkDynamic} {
		out := t.TempDir()
		p, _, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
		if err != nil {
			t.Fatal(err)
		}
		g, err := p.Run(driver.HostTarget(), nil, out)
		if err != nil {
			t.Fatal(err)
		}
		rg, err := driver.ResolveGraph(testCompiler(), g, nil)
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		if err := driver.ExecuteGraph(p, rg, out, driver.ExecOptions{Link: link}, &stderr); err != nil {
			t.Fatalf("link %v: %v\n%s", link, err, stderr.String())
		}
		if got := runExe(t, filepath.Join(out, "app")); got != "8192 42\n" {
			t.Errorf("link %v: output %q", link, got)
		}
	}
}
