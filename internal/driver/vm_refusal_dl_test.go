package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// TestVMRefusalHostDL checks host.dl() refuses as native-only, not as a gap --engine interp closes.
func TestVMRefusalHostDL(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"dlrefusal\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	src := `let loader = host.dl()
let lib = loader.open("libc.so.6")?
print "opened"
`
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	_, err = driver.RunWith(m, driver.RunOptions{Engine: "vm"}, &out, &errOut, nil)
	if err == nil {
		t.Fatal("want a VM refusal")
	}
	if !strings.Contains(err.Error(), "primitive os.Host.dl") || !strings.Contains(err.Error(), "native-only") || !strings.Contains(err.Error(), "kigumi build") {
		t.Errorf("want a native-only `kigumi build` refusal naming os.Host.dl, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "--engine interp") {
		t.Errorf("must not point at --engine interp: interp only panics later, inside Loader.open, got %q", err.Error())
	}
}
