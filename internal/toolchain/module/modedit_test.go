package module_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/version"
)

func TestManifestEdit(t *testing.T) {
	root := t.TempDir()
	if err := module.WriteManifest(root, "github.com/acme/app"); err != nil {
		t.Fatal(err)
	}
	if err := module.WriteManifest(root, "github.com/acme/app"); err == nil {
		t.Fatal("second init should refuse to overwrite")
	}
	mf, _, err := module.ReadModFile(root)
	if err != nil || mf.Kigumi != version.Toolchain {
		t.Fatalf("init should record the toolchain version: %+v %v", mf, err)
	}
	if mf.Zig != version.Zig {
		t.Fatalf("init should pin the current default zig version: %+v", mf)
	}
	os.WriteFile(filepath.Join(root, "mod.kg"), manifest("app", "\n// keep me\nRequire {\n    name: \"a\"\n    version: \"v1.0.0\"\n    url: \"u\"\n}\n"), 0o644)
	if err := module.AppendRequire(root, module.Require{Name: "github.com/acme/b", Version: "v2.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := module.AppendRequire(root, module.Require{Name: "github.com/acme/b", Version: "v3.0.0"}); err == nil {
		t.Fatal("duplicate require should be refused")
	}
	os.MkdirAll(filepath.Join(root, "..", "b"), 0o755)
	if err := module.AppendLocalReplace(root, "github.com/acme/b", "../b"); err != nil {
		t.Fatal(err)
	}
	local, err := module.ReadLocal(buildconfig.CompilerOptions{Cwd: root}, filepath.Join(root, "deep", "er"))
	if err != nil || local.Replaces["github.com/acme/b"] != filepath.Join(root, "../b") {
		t.Fatalf("local override: %+v %v", local, err)
	}
	if local, err = module.ReadLocal(buildconfig.CompilerOptions{Cwd: root}, filepath.Join("deep", "er")); err != nil || local.Replaces["github.com/acme/b"] != filepath.Join(root, "../b") {
		t.Fatalf("local override from a relative dir: %+v %v", local, err)
	}
	if err := module.RemoveRequires(root, map[string]bool{"a": true}); err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(filepath.Join(root, "mod.kg"))
	if strings.Contains(string(src), "// keep me") || strings.Contains(string(src), "\"a\"") || !strings.Contains(string(src), "acme/b") {
		t.Fatalf("remove kept the wrong lines:\n%s", src)
	}

	script := filepath.Join(root, "tool.kg")
	os.WriteFile(script, []byte("#!/usr/bin/env kigumi\n\n// a script\nimport {hello} from github.com/acme/greet\nprint hello()\n"), 0o644)
	if err := module.AppendScriptRequire(script, module.Require{Name: "github.com/acme/greet", Version: "v0.1.0"}); err != nil {
		t.Fatal(err)
	}
	if err := module.AppendScriptRequire(script, module.Require{Name: "github.com/acme/other", Version: "v0.2.0"}); err != nil {
		t.Fatal(err)
	}
	mf, has, err := module.ReadScriptManifest(script)
	if err != nil || !has || mf.Kigumi != version.Toolchain || len(mf.Requires) != 2 || mf.Requires[1].Name != "github.com/acme/other" {
		t.Fatalf("script manifest: %+v %v %v", mf, has, err)
	}
	src, _ = os.ReadFile(script)
	if !strings.HasPrefix(string(src), "#!/usr/bin/env kigumi\n\n// a script\nKigumi { version:") || !strings.Contains(string(src), "}\nRequire {") {
		t.Fatalf("records should sit after the shebang and comments:\n%s", src)
	}
}
