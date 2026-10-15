package driver_test

import (
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func TestCompilerOptionsHostDecidesTheDefaultTarget(t *testing.T) {
	darwin := driver.CompilerOptions{Host: driver.Target{OS: "darwin", Arch: "arm64", Sys: "posix"}}
	got, err := driver.ParseTarget(darwin, "", "")
	if err != nil || got.OS != "darwin" || got.Arch != "arm64" {
		t.Fatalf("ParseTarget with a darwin host = %+v, %v", got, err)
	}
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	writeStable(t, root, map[string]string{
		"mod.kg":          stableFiles["mod.kg"],
		"main.kg":         "print \"hi\"\n",
		"which_darwin.kg": "fn which() -> String { \"darwin\" }\n",
		"which_linux.kg":  "fn which() -> String { \"linux\" }\n",
	})
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Compiler: darwin})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, f := range m.Packages["stable"].Files {
		files = append(files, f.File.Name)
	}
	if strings.Join(files, ",") != "main.kg,which_darwin.kg" {
		t.Errorf("a darwin host selected %v", files)
	}
}

func TestCompilerOptionsCacheDirIsExplicit(t *testing.T) {
	cache := t.TempDir()
	got, err := driver.CacheDir(driver.CompilerOptions{CacheDir: cache}, driver.Require{Name: "example.com/a", Version: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cache, "kigumi", "mod", "example.com", "a@v1.0.0"); got != want {
		t.Errorf("CacheDir = %q, want %q", got, want)
	}
	header, _ := driver.HeaderCacheDir(driver.CompilerOptions{CacheDir: cache})
	derive, _ := driver.DeriveCacheDir(driver.CompilerOptions{CacheDir: cache})
	if header != filepath.Join(cache, "kigumi", "cheader") || derive != filepath.Join(cache, "kigumi", "derive") {
		t.Errorf("header cache %q, derive cache %q", header, derive)
	}
}
