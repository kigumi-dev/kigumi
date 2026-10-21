package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func TestAotLinkNameFunctions(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	src := `export(C) {
    @linkName("plugin_add")
    pub fn add(a: i32, b: i32) -> i32 {
        a + b
    }
}

extern(C) {
    @linkName("host_triple")
    fn triple(x: i32) -> i32

    @linkName("call_plugin_add")
    fn callAdd(a: i32, b: i32) -> i32
}

print "triple=${unsafe { triple(14) }}"
print "add=${unsafe { callAdd(40, 2) }}"
`
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	helper := filepath.Join(root, "helper.c")
	cHelper := "" +
		"int plugin_add(int a, int b);\n" +
		"int host_triple(int x) { return x * 3; }\n" +
		"int call_plugin_add(int a, int b) { return plugin_add(a, b); }\n"
	os.WriteFile(helper, []byte(cHelper), 0o644)
	t.Setenv("KIGUMI_CFLAGS", helper)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if want := "triple=42\nadd=42\n"; string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}
