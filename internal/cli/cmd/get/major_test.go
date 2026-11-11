package get_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// A /v2 module is a separate path: `get` picks its newest v2 tag (not a v1
// one), fetches the versioned subdirectory, and a run resolves imports
// through it.
func TestGetMajorSuffixModule(t *testing.T) {
	root := t.TempDir()
	repo := clitest.Monorepo(t, root)
	os.MkdirAll(filepath.Join(repo, "sgx", "v2", "enclave"), 0o755)
	os.WriteFile(filepath.Join(repo, "sgx", "v2", "mod.kg"), clitest.Manifest("github.com/kigumilang/exp/sgx/v2", ""), 0o644)
	os.WriteFile(filepath.Join(repo, "sgx", "v2", "enclave", "enclave.kg"), []byte("pub fn hello() -> String {\n    \"v2\"\n}\n"), 0o644)
	clitest.Git(t, repo, "add", ".")
	clitest.Git(t, repo, "commit", "-q", "-m", "v2 module")
	clitest.Git(t, repo, "tag", "sgx/v2/v2.0.0")
	// A newer v1 tag must not be offered for the /v2 path.
	clitest.Git(t, repo, "tag", "sgx/v1.9.0")

	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", ""), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import enclave from github.com/kigumilang/exp/sgx/v2/enclave\nprint enclave.hello()\n"), 0o644)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	code, out, errOut := clitest.Execute(t, "get", "github.com/kigumilang/exp/sgx/v2", "--url", "file://"+repo, app)
	if code != 0 || !strings.Contains(errOut, "added github.com/kigumilang/exp/sgx/v2 v2.0.0") {
		t.Fatalf("get /v2: exit %d %q %q", code, out, errOut)
	}
	mf, _ := os.ReadFile(filepath.Join(app, "mod.kg"))
	if !strings.Contains(string(mf), "version: \"v2.0.0\"") {
		t.Fatalf("manifest after get:\n%s", mf)
	}
	cached := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "kigumi", "mod", "github.com", "kigumilang", "exp", "sgx", "v2@v2.0.0")
	if _, err := os.Stat(filepath.Join(cached, "enclave", "enclave.kg")); err != nil {
		t.Fatalf("the /v2 subdirectory was not extracted: %v", err)
	}
	std, _ := filepath.Abs("../../../../std")
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "v2\n" {
		t.Fatalf("run /v2: exit %d out %q err %q", code, out, errOut)
	}
}
