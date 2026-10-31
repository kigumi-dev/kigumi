package driver_test

import (
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

func TestCompilerOptionsHostDecidesTheDefaultTarget(t *testing.T) {
	darwin := buildconfig.CompilerOptions{Host: buildconfig.Target{OS: "darwin", Arch: "arm64", Sys: "posix"}}
	got, err := buildconfig.ParseTarget(darwin, "", "")
	if err != nil || got.OS != "darwin" || got.Arch != "arm64" {
		t.Fatalf("ParseTarget with a darwin host = %+v, %v", got, err)
	}
	std, _ := filepath.Abs("../../../std")
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

func TestZeroCompilerOptionsReadsNothingFromTheProcess(t *testing.T) {
	var zero buildconfig.CompilerOptions
	if _, err := zero.Abs("rel"); err == nil {
		t.Error("Abs of a relative path with an empty Cwd must fail")
	}
	if p, err := zero.Abs("/abs/x"); err != nil || p != "/abs/x" {
		t.Errorf("Abs of an absolute path = %q, %v", p, err)
	}
	for _, o := range []buildconfig.CompilerOptions{zero, {PathDirs: []string{}}} {
		if p, err := o.ToolLookup()("sh"); err == nil {
			t.Errorf("an empty search path found sh at %s", p)
		}
		if o.CCompiler() != nil {
			t.Error("an empty search path found a C compiler")
		}
	}
	if _, err := zero.ZigStoreRoot(""); err == nil {
		t.Error("ZigStoreRoot with no Home and no override must fail")
	}
	if got, err := zero.ZigStoreRoot("/store"); err != nil || got != "/store" {
		t.Errorf("ZigStoreRoot override = %q, %v", got, err)
	}
	if _, err := driver.CacheDir(zero, driver.Require{Name: "example.com/a", Version: "v1.0.0"}); err == nil {
		t.Error("CacheDir with an empty CacheDir must fail")
	}
	if _, err := driver.HeaderCacheDir(zero); err == nil {
		t.Error("HeaderCacheDir with an empty CacheDir must fail")
	}
	if _, err := driver.LoadModule("relative", driver.LoadOptions{}); err == nil {
		t.Error("a relative module root with an empty Cwd must fail")
	}
}

func TestCompilerOptionsCacheDirIsExplicit(t *testing.T) {
	cache := t.TempDir()
	got, err := driver.CacheDir(buildconfig.CompilerOptions{CacheDir: cache}, driver.Require{Name: "example.com/a", Version: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cache, "kigumi", "mod", "example.com", "a@v1.0.0"); got != want {
		t.Errorf("CacheDir = %q, want %q", got, want)
	}
	header, _ := driver.HeaderCacheDir(buildconfig.CompilerOptions{CacheDir: cache})
	derive, _ := driver.DeriveCacheDir(buildconfig.CompilerOptions{CacheDir: cache})
	if header != filepath.Join(cache, "kigumi", "cheader") || derive != filepath.Join(cache, "kigumi", "derive") {
		t.Errorf("header cache %q, derive cache %q", header, derive)
	}
}
