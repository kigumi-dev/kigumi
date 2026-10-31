package version

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestZigPinSingleSource(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	file, err := os.ReadFile(filepath.Join(root, "ZIG_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(file)); got != Zig {
		t.Errorf("ZIG_VERSION pins %q, version.Zig is %q", got, Zig)
	}
	mods, _ := filepath.Glob(filepath.Join(root, "examples", "*", "mod.kg"))
	if len(mods) == 0 {
		t.Fatal("no examples/*/mod.kg found")
	}
	pin := regexp.MustCompile(`zig:\s*"([^"]*)"`)
	for _, m := range mods {
		src, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		found := pin.FindSubmatch(src)
		if found == nil {
			t.Errorf("%s has no zig pin", m)
			continue
		}
		if string(found[1]) != Zig {
			t.Errorf("%s pins zig %q, version.Zig is %q", m, found[1], Zig)
		}
	}
}
