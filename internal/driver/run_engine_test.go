package driver_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/testtool"
)

// Expected engines and stderr policy come from testdata/manifest.json; nothing falls back to another engine.
func TestRunCorpusEngines(t *testing.T) {
	manifest := loadManifest(t)
	files, _ := filepath.Glob("../../testdata/run/*.txtar")
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	want, ran := map[string]int{}, map[string]int{}
	for _, path := range files {
		id := fixtureID(path)
		fx := manifest.ByID(id)
		if fx == nil {
			t.Fatalf("%s: no manifest entry (run UPDATE=1 go test -run TestManifestFresh ./internal/driver/)", id)
		}
		var engines []string
		for _, e := range []string{"vm", "interp"} {
			if fx.Runs(e) {
				engines = append(engines, e)
				want[e]++
			}
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			if len(engines) == 0 {
				t.Skipf("manifest: %s only runs native (%v)", id, fx.Skip)
			}
			root, wantOut, wantErr, wantExit := loadRunFixture(t, path)
			m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: "../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"main/main.kg", "one", "two"}
			for _, engine := range engines {
				t.Run(engine, func(t *testing.T) {
					t.Chdir(t.TempDir())
					var out, errOut bytes.Buffer
					code, err := driver.RunWith(m, driver.RunOptions{Engine: engine}, &out, &errOut, args)
					if err != nil {
						t.Fatalf("manifest says %s runs %s, but it refused: %v", engine, id, err)
					}
					ran[engine]++
					if out.String() != wantOut {
						t.Errorf("stdout mismatch\n--- got ---\n%s--- want ---\n%s", out.String(), wantOut)
					}
					if err := testtool.CheckStderr(fx.Stderr, wantErr, errOut.String()); err != nil {
						t.Error(err)
					}
					if code != wantExit {
						t.Errorf("exit %d, want %d\nstderr:\n%s", code, wantExit, errOut.String())
					}
				})
			}
		})
	}
	t.Logf("vm: expect %d, ran %d", want["vm"], ran["vm"])
	t.Logf("interp: expect %d, ran %d", want["interp"], ran["interp"])
}

// TestIsrEngineMatrix runs isr.txtar on all three engines, since it never reaches the VM's refusal list for TestRunCorpusEngines to fall back on.
func TestIsrEngineMatrix(t *testing.T) {
	runEngineMatrix(t, "../../testdata/run/isr.txtar")
}

// TestLetWildcardDropEngineMatrix guards let_wildcard_drop.txtar the same way, for the same reason.
func TestLetWildcardDropEngineMatrix(t *testing.T) {
	runEngineMatrix(t, "../../testdata/run/let_wildcard_drop.txtar")
}

// TestDiscardPlaceDropEngineMatrix checks discarding an already-bound local drops it exactly once, not again at its own scope exit.
func TestDiscardPlaceDropEngineMatrix(t *testing.T) {
	runEngineMatrix(t, "../../testdata/run/discard_place_drop.txtar")
}

// TestCustomIndexWitnessOrderEngineMatrix checks a witnessed bracket read evaluates its receiver before its index on every engine.
func TestCustomIndexWitnessOrderEngineMatrix(t *testing.T) {
	runEngineMatrix(t, "../../testdata/run/custom_index_witness_order.txtar")
}

// runEngineMatrix runs one testdata/run fixture on interp, VM and (if a C compiler exists) native AOT, and checks all three agree.
func runEngineMatrix(t *testing.T, path string) {
	t.Helper()
	root, wantOut, _, wantExit := loadRunFixture(t, path)
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: "../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range []string{"interp", "vm"} {
		t.Run(engine, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var out, errOut bytes.Buffer
			code, err := driver.RunWith(m, driver.RunOptions{Engine: engine}, &out, &errOut, nil)
			if err != nil {
				t.Fatalf("%s: %v", engine, err)
			}
			if out.String() != wantOut {
				t.Errorf("%s: stdout mismatch\n--- got ---\n%s--- want ---\n%s", engine, out.String(), wantOut)
			}
			if code != wantExit {
				t.Errorf("%s: exit %d, want %d\nstderr:\n%s", engine, code, wantExit, errOut.String())
			}
		})
	}
	t.Run("native", func(t *testing.T) {
		if driver.CCompiler() == nil {
			t.Skip("no C compiler")
		}
		runAot(t, path, loadManifest(t).ByID(fixtureID(path)))
	})
}
