package clitest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/cli/session"
)

const modHead = "Module {\n    name: \"%s\"\n    kigumi: \"0.1\"\n}\n"

// Manifest is a mod.kg naming the module, followed by rest.
func Manifest(name, rest string) []byte {
	return []byte(strings.Replace(modHead, "%s", name, 1) + rest)
}

// Git runs git in dir with a fixed identity and skips the test when git is unavailable.
func Git(t *testing.T, dir string, args ...string) string {
	git, err := session.CompilerOptions().LookTool("git")
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	cmd := exec.Command(git, append([]string{"-c", "user.email=t@example.com", "-c", "user.name=t"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// Monorepo builds a repo whose checkout path ends in the module's host path, so a Require can point its url at it.
func Monorepo(t *testing.T, root string) string {
	repo := filepath.Join(root, "github.com", "kigumilang", "exp")
	os.MkdirAll(filepath.Join(repo, "sgx", "enclave"), 0o755)
	os.MkdirAll(filepath.Join(repo, "web"), 0o755)
	os.WriteFile(filepath.Join(repo, "sgx", "mod.kg"), Manifest("github.com/kigumilang/exp/sgx", ""), 0o644)
	os.WriteFile(filepath.Join(repo, "sgx", "enclave", "enclave.kg"), []byte("pub fn hello() -> String {\n    \"hi\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "web", "mod.kg"), Manifest("github.com/kigumilang/exp/web", ""), 0o644)
	os.WriteFile(filepath.Join(repo, "web", "web.kg"), []byte("pub fn serve() -> Int {\n    1\n}\n"), 0o644)
	Git(t, repo, "init", "-q")
	Git(t, repo, "add", ".")
	Git(t, repo, "commit", "-q", "-m", "initial")
	Git(t, repo, "tag", "sgx/v0.1.0")
	Git(t, repo, "tag", "web/v0.3.0")
	return repo
}

// WriteFiles writes each root-relative file and sets its mtime to stamp.
func WriteFiles(t *testing.T, root string, files map[string]string, stamp time.Time) {
	t.Helper()
	for rel, body := range files {
		full := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(full, stamp, stamp)
	}
}
