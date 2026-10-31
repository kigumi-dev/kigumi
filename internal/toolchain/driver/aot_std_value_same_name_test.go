package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const sameNameMain = `import ffi from std/ffi
import pa from samename/pa
import pb from samename/pb

let f = ffi.sizeOf[pa.Rec]
let g = ffi.sizeOf[pb.Rec]
print "${f(&pa.Rec { a: 1 })} ${g(&pb.Rec { a: 1, b: 2 })}"
`

func TestAotStdValueSameNamedTypesKeepTheirOwnAdapters(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	files := map[string]string{
		"mod.kg":          "Module {\n    name: \"samename\"\n    kigumi: \"0.1\"\n}\n",
		"pa/pa.kg":        "pub type Rec layout(C) = {\n    pub a i32\n}\n",
		"pb/pb.kg":        "pub type Rec layout(C) = {\n    pub a i64\n    pub b i64\n}\n",
		"cmd/app/main.kg": sameNameMain,
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(body), 0o644)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	var vmOut, vmErr bytes.Buffer
	if code, err := driver.Run(m, &vmOut, &vmErr, nil); err != nil || code != 0 {
		t.Fatalf("run: %v %d\n%s", err, code, vmErr.String())
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if string(out) != "4 16\n" || vmOut.String() != "4 16\n" {
		t.Errorf("native %q, vm %q, want \"4 16\\n\"", out, vmOut.String())
	}
}
