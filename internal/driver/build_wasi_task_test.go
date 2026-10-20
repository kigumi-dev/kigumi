package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

const wasiTaskProgram = `import task from std/task

async fn boom() -> Int {
    None.expect("x")
}

async fn root() -> Int {
    await task.spawn(boom())
}

let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(root())}"
`

func TestBuildWasiTaskProgram(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("needs a C compiler")
	}
	target, err := driver.ParseTarget(testCompiler(), "wasm32-wasi", "")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"w\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(wasiTaskProgram), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../std", Target: target})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ok, err := driver.BuildWith(m, filepath.Join(root, "out.wasm"), &stderr, driver.BuildOptions{Target: target})
	if err != nil || !ok {
		t.Fatalf("wasm32-wasi build: %v\n%s", err, stderr.String())
	}
}
