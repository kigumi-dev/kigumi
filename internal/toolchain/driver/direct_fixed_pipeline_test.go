package driver_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/toolchain/driver"
)

func pipelineIR(t *testing.T, src string) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"probe\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatalf("check errors:\n%s", a.Res.Render())
	}
	return llgen.EmitProgram(a.Prog, llgen.EmitOptions{}).IR
}

func irBody(t *testing.T, ir, sym string) string {
	t.Helper()
	loc := regexp.MustCompile(`define \S+ @"` + regexp.QuoteMeta(sym) + `"\(`).FindStringIndex(ir)
	if loc == nil {
		t.Fatalf("no definition of %s in IR", sym)
	}
	body, _, _ := strings.Cut(ir[loc[0]:], "\n}\n")
	return body
}

const fixedPipelineSrc = `
fn work(seed: Int) -> Int {
    let mut fa = FixedArray.filled[Int, 4](seed)
    fa[1] = fa[0] + 10
    let mut sum = 0
    for x in fa {
        sum = sum + x
    }
    sum + (fa.len().toInt() || 0) + (fa.get(9) || 0)
}

fn main() -> Void! { print "${work(1)}" }
`

func TestFixedArrayStaysInlineThroughTheDriverPipeline(t *testing.T) {
	body := irBody(t, pipelineIR(t, fixedPipelineSrc), "probe/main.work")
	for _, bad := range []string{"@rt_index(", "@rt_setindex(", "@rt_builtin(", `@"kg.box.`, `@"kg.unbox.`} {
		if strings.Contains(body, bad) {
			t.Errorf("probe/main.work calls %s after the driver's pass order", bad)
		}
	}
}
