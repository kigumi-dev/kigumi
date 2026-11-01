package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/cli/clitest"
)

// Codifies the rule .github/workflows/kigumi-test.yml's build step applies: a build.kg module takes --out-dir, a plain module takes -o.
func TestReusableWorkflowPicksBuildFlagByModuleMode(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	plain := t.TempDir()
	os.WriteFile(filepath.Join(plain, "mod.kg"), clitest.Manifest("plainmod", ""), 0o644)
	os.WriteFile(filepath.Join(plain, "main.kg"), []byte("print \"hi\"\n"), 0o644)

	bp := t.TempDir()
	os.WriteFile(filepath.Join(bp, "mod.kg"), clitest.Manifest("bpmod", ""), 0o644)
	os.WriteFile(filepath.Join(bp, "build.kg"), []byte("import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let a = b.executable(\"app\", \"\", b.request().target())?\n}\n"), 0o644)
	os.WriteFile(filepath.Join(bp, "main.kg"), []byte("print \"hi\"\n"), 0o644)

	for _, c := range []struct {
		name, root, rightFlag, wrongFlag string
	}{
		{"plain module", plain, "-o", "--out-dir"},
		{"build.kg module", bp, "--out-dir", "-o"},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := t.TempDir()
			target := out + "/a.out"
			if c.rightFlag == "--out-dir" {
				target = out
			}
			if code, _, errOut := clitest.Execute(t, "--std", std, "build", c.root, c.rightFlag, target); code != 0 {
				t.Fatalf("%s %s: exit %d\n%s", c.rightFlag, target, code, errOut)
			}
			wrongTarget := out + "-wrong/a.out"
			if c.wrongFlag == "--out-dir" {
				wrongTarget = out + "-wrong"
			}
			if code, _, errOut := clitest.Execute(t, "--std", std, "build", c.root, c.wrongFlag, wrongTarget); code == 0 {
				t.Fatalf("%s should reject %s:\n%s", c.name, c.wrongFlag, errOut)
			}
		})
	}
}
