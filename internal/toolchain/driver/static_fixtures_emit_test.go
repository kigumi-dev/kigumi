package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// TestAotCorpus builds these too, but there a backstop panic in llgen crashes the whole test binary instead of failing one fixture.
func TestStaticFixturesEmitNatively(t *testing.T) {
	manifest := loadManifest(t)
	files, _ := filepath.Glob("../../../testdata/run/*.txtar")
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fx := manifest.ByID(fixtureID(path))
		if !bytes.Contains(data, []byte("static ")) || fx == nil || !fx.Runs("native") {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			root, _, _, _ := loadRunFixture(t, path)
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			a, err := driver.Analyze(m)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Res.StaticEntities()) == 0 {
				t.Skip("declares no static")
			}
			emitWithoutPanic(t, m)
		})
	}
}

func emitWithoutPanic(t *testing.T, m *driver.Module) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("native emission panicked: %v", p)
		}
	}()
	var stderr bytes.Buffer
	if _, ok, err := driver.Compile(m, &stderr); err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
}
