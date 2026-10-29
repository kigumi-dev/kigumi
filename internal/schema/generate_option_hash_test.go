package schema_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

func TestGeneratedOptionHashIncludesPayload(t *testing.T) {
	m, _ := loadModule(t, `
let a = Some("a")
let b = Some("b")
print "${hash(&a) == hash(&b)} ${a == b}"
print "${hash(&Some("a")) == hash(&Some("b"))}"
`)
	a, err := driver.Analyze(m)
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v", err)
	}
	var out, errOut bytes.Buffer
	if _, err := vm.Run(a.Prog, vm.Options{}, &out, &errOut); err != nil {
		t.Fatalf("run: %v\n%s", err, errOut.String())
	}
	if out.String() != "false false\nfalse\n" {
		for _, generated := range a.Res.GeneratedSources() {
			t.Log(generated.Text)
		}
		t.Fatalf("output = %q, want distinct hashes and unequal values", out.String())
	}
	t.Run("native", func(t *testing.T) {
		if testCompiler().CCompiler() == nil {
			t.Skip("no C compiler")
		}
		exe := filepath.Join(t.TempDir(), "option_hash")
		var diagnostics bytes.Buffer
		if ok, err := driver.Build(m, exe, &diagnostics); err != nil || !ok {
			t.Fatalf("build: %v\n%s", err, diagnostics.String())
		}
		output, err := exec.Command(exe).CombinedOutput()
		if err != nil || string(output) != out.String() {
			t.Fatalf("native output = %q, err = %v; VM output = %q", output, err, out.String())
		}
	})
}
