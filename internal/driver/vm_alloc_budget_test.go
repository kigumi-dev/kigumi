package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

const hugeAllocSrc = `let zeros = Bytes.tryZeros(137438953472)
let filled = Bytes.tryFill(7, 137438953472)
let cap = Array.tryWithCapacity[Int](137438953472)
let e = host.entropy()
let random = e.bytes(137438953472)
print "${zeros is Err(AllocError.OutOfMemory)} ${filled is Err(AllocError.OutOfMemory)} ${cap is Err(AllocError.OutOfMemory)} ${random is Err(_)}"
`

// Go's out-of-memory error is fatal, so every size-taking primitive must refuse a 2^37 request itself.
func TestVMHugeSizesReturnErr(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main/main.kg"), []byte(hugeAllocSrc), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.Run(m, &out, &errOut, nil)
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v\n%s%s", code, err, out.String(), errOut.String())
	}
	if out.String() != "true true true true\n" {
		t.Errorf("stdout = %q", out.String())
	}
}
