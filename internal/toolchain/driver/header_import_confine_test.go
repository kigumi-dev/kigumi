package driver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func loadHeaderModule(t *testing.T, files map[string]string, links map[string]string) error {
	t.Helper()
	return loadHeaderModuleAt(t, files, links, false)
}

func loadHeaderModuleAt(t *testing.T, files map[string]string, links map[string]string, relative bool) error {
	t.Helper()
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("no zig")
	}
	top := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(top, "cache"))
	root := filepath.Join(top, "mod")
	files["mod.kg"] = "Module {\n    name: \"hdr\"\n    kigumi: \"0.1\"\n}\nHeader {\n    path: \"inc/a.h\"\n    package: \"a\"\n}\n"
	files["main.kg"] = "print 1\n"
	for name, body := range files {
		p := filepath.Join(top, name)
		if !strings.HasPrefix(name, "outside/") {
			p = filepath.Join(root, name)
		}
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range links {
		if err := os.Symlink(filepath.Join(top, target), filepath.Join(root, name)); err != nil {
			t.Skip(err)
		}
	}
	std, _ := filepath.Abs("../../../std")
	if relative {
		t.Chdir(root)
		root = "."
	}
	_, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	return err
}

func TestHeaderImportRefusesIncludeOutsideModule(t *testing.T) {
	for name, tc := range map[string]struct {
		files, links map[string]string
	}{
		"dotdot":  {files: map[string]string{"inc/a.h": "#include \"../../outside/o.h\"\n", "outside/o.h": "int o(void);\n"}},
		"symlink": {files: map[string]string{"inc/a.h": "#include \"o.h\"\n", "outside/o.h": "int o(void);\n"}, links: map[string]string{"inc/o.h": "outside/o.h"}},
		"embed":   {files: map[string]string{"inc/a.h": "static const unsigned char k[] = {\n#embed \"../../outside/o.h\"\n};\n", "outside/o.h": "xx"}},
	} {
		err := loadHeaderModule(t, tc.files, tc.links)
		if err == nil || !strings.Contains(err.Error(), "outside the module") {
			t.Errorf("%s: want an outside-the-module error, got %v", name, err)
		}
	}
}

func TestHeaderImportAllowsInsideAndSystemIncludes(t *testing.T) {
	err := loadHeaderModule(t, map[string]string{
		"inc/a.h": "#include <stdint.h>\n#include \"b.h\"\nint32_t a(b_t x);\n",
		"inc/b.h": "typedef int b_t;\n",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestHeaderImportAllowsInsideIncludesFromRelativeRoot(t *testing.T) {
	files := map[string]string{"inc/a.h": "#include \"b.h\"\nint a(b_t x);\n", "inc/b.h": "typedef int b_t;\n"}
	if err := loadHeaderModuleAt(t, files, nil, true); err != nil {
		t.Fatal(err)
	}
	files = map[string]string{"inc/a.h": "#include \"../../outside/o.h\"\n", "outside/o.h": "int o(void);\n"}
	if err := loadHeaderModuleAt(t, files, nil, true); err == nil || !strings.Contains(err.Error(), "outside the module") {
		t.Errorf("relative root, escaping include: want a refusal, got %v", err)
	}
}
