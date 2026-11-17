package mod_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestModRenameKeepsNestedModules(t *testing.T) {
	app := writeRenameModule(t, t.TempDir())
	mf := clitest.Manifest("github.com/acme/app", `Require { name: "github.com/acme/dep", version: "v1.0.0" }
Require { name: "github.com/acme/dep/v2", version: "v2.0.0" }
Require { name: "example.com/moved/dep/v2", version: "v2.0.0" }
`)
	if err := os.WriteFile(filepath.Join(app, "mod.kg"), mf, 0o644); err != nil {
		t.Fatal(err)
	}
	src := `import a from github.com/acme/dep/sub
import b from github.com/acme/dep/v2/sub
import c from example.com/moved/dep/v2/sub
`
	if err := os.WriteFile(filepath.Join(app, "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := clitest.Execute(t, "mod", "rename", "-C", app, "github.com/acme/dep", "example.com/moved/dep")
	if code != 0 {
		t.Fatalf("rename: %s", errOut)
	}
	want := strings.Replace(src, "from github.com/acme/dep/sub", "from example.com/moved/dep/sub", 1)
	if got := readFile(t, filepath.Join(app, "main.kg")); got != want {
		t.Fatalf("imports = %q, want %q", got, want)
	}
	for _, name := range []string{"github.com/acme/dep/v2", "example.com/moved/dep/v2"} {
		if got := readFile(t, filepath.Join(app, "mod.kg")); !strings.Contains(got, `name: "`+name+`"`) {
			t.Fatalf("missing module %s:\n%s", name, got)
		}
	}
}
