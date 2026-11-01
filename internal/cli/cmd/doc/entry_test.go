package doc_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/cli/clitest"
)

func TestDocOfAnEntryFileDescribesTheWholeModule(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	root := t.TempDir()
	t.Setenv("KIGUMI_PKG_CACHE", t.TempDir())
	clitest.WriteFiles(t, root, map[string]string{
		"mod.kg":          "Module {\n    name: \"settle\"\n    kigumi: \"0.1\"\n}\n",
		"lib/lib.kg":      "// one is the number one.\npub fn one() -> Int {\n    1\n}\n",
		"extra/extra.kg":  "// two is the number two.\npub fn two() -> Int {\n    2\n}\n",
		"cmd/app/main.kg": "import lib from settle/lib\n\nprint \"${lib.one()}\"\n",
	}, time.Now().Add(-time.Hour))
	_, whole, _ := clitest.Execute(t, "--std", std, "doc", root)
	code, out, errOut := clitest.Execute(t, "--std", std, "doc", filepath.Join(root, "cmd/app/main.kg"))
	if code != 0 || out != whole {
		t.Fatalf("doc of an entry file must equal doc of the module: exit %d, stderr %q\n%q\nvs\n%q", code, errOut, out, whole)
	}
	if !strings.Contains(out, "settle/extra") {
		t.Errorf("the package the entry does not import must be documented: %q", out)
	}
}
