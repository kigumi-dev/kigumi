package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	devfixture "kigumi/internal/devkit/fixture"
	"kigumi/internal/toolchain/driver"
)

// TestAotCorpus builds these too, but there a backstop panic in llgen crashes the whole test binary instead of failing one fixture.
func TestStaticFixturesEmitNatively(t *testing.T) {
	emitFixturesNatively(t, "static ", func(r *sem.Result) bool { return len(r.StaticEntities()) > 0 })
}

func TestConstFixturesEmitNatively(t *testing.T) {
	emitFixturesNatively(t, "const ", declaresConst)
}

func emitFixturesNatively(t *testing.T, marker string, declares func(*sem.Result) bool) {
	manifest := loadManifest(t)
	files, _ := filepath.Glob("../../../testdata/run/*.txtar")
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	for _, path := range files {
		if !devfixture.Shard(filepath.Base(path)) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fx := manifest.ByID(fixtureID(path))
		if !bytes.Contains(data, []byte(marker)) || fx == nil || !fx.Runs("native") {
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
			if !declares(a.Res) {
				t.Skip("declares no " + strings.TrimSpace(marker))
			}
			emitWithoutPanic(t, m)
		})
	}
}

func declaresConst(r *sem.Result) bool {
	for id := 1; id < len(r.Entities); id++ {
		e := r.Entity(sem.EntityID(id))
		if e.Kind == sem.EntConst && !r.Packages[e.Pkg].Std {
			return true
		}
	}
	return false
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
