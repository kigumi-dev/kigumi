package driver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestCorruptRuntimeABISidecarFailsClosed(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "libprobe.a")
	if err := os.WriteFile(lib, []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lib+".kgabi.json", []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Module{Links: []module.Link{{Library: "probe", Search: dir}}}
	err := checkLinkedRuntimeABI(m, nil, nil, "kgabi1:abc")
	if err == nil || !strings.Contains(err.Error(), "runtime-abi-unreadable") {
		t.Fatalf("a corrupt sidecar must not fail open: %v", err)
	}
}
