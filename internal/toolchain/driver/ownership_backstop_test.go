package driver_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mirverify "kigumi/internal/compiler/mir/verify"
	devfixture "kigumi/internal/devkit/fixture"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

var semOnlyOptions = []string{"pending", "std", "deps", "blocking", "target", "ptr"}

// TestCorpusOwnershipBeforeVerifier: the verifier, ownership rule included, is a backstop no corpus program reaches.
func TestCorpusOwnershipBeforeVerifier(t *testing.T) {
	semPaths, _ := filepath.Glob("../../../testdata/sem/*.txtar")
	runPaths, _ := filepath.Glob("../../../testdata/run/*.txtar")
	if len(semPaths) == 0 || len(runPaths) == 0 {
		t.Fatal("no sem or run fixtures")
	}
	for _, path := range semPaths {
		if !devfixture.Shard(filepath.Base(path)) {
			continue
		}
		t.Run("sem/"+strings.TrimSuffix(filepath.Base(path), ".txtar"), func(t *testing.T) {
			t.Parallel()
			fx := parseSemDriverFixture(t, path)
			if why, ok := semOnly(fx); ok {
				t.Skip(why)
			}
			root, opts := writeSemModule(t, fx)
			m, err := driver.LoadModule(root, opts)
			if err != nil {
				t.Skipf("not a driver module: %v", err)
			}
			checkOwnershipBackstop(t, m)
		})
	}
	manifest := loadManifest(t)
	for _, path := range runPaths {
		if !devfixture.Shard(filepath.Base(path)) {
			continue
		}
		id := fixtureID(path)
		fx := manifest.ByID(id)
		if fx == nil {
			t.Fatalf("%s: no manifest entry (run UPDATE=1 go test -run TestManifestFresh ./internal/toolchain/driver/)", id)
		}
		// A fixture the VM runs already takes the same check through
		// TestRunCorpusEngines; only the manifest's VM exclusions are unique here.
		if fx.Runs("vm") {
			continue
		}
		t.Run("run/"+strings.TrimSuffix(filepath.Base(path), ".txtar"), func(t *testing.T) {
			t.Parallel()
			root, _, _, _ := loadRunFixture(t, path)
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			checkOwnershipBackstop(t, m)
		})
	}
}

func semOnly(fx semDriverFixture) (string, bool) {
	for _, k := range semOnlyOptions {
		if _, ok := fx.opts[k]; ok {
			return "sem-only option " + k, true
		}
	}
	for name := range fx.files {
		if strings.HasPrefix(name, "std/") {
			return "replaces std file " + name, true
		}
	}
	return "", false
}

func checkOwnershipBackstop(t *testing.T, m *driver.Module) {
	t.Helper()
	_, err := driver.Check(m)
	if _, ok := errors.AsType[*mirverify.Error](err); ok {
		t.Errorf("the verifier caught what the checks before it let through: %v", err)
	}
}

// writeSemModule puts src where the sem corpus checks it: package main, or the root for a driver-stage fixture.
func writeSemModule(t *testing.T, fx semDriverFixture) (string, driver.LoadOptions) {
	t.Helper()
	root := t.TempDir()
	opts := driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Target: buildconfig.HostTarget()}
	if triple := fx.opts["triple"]; triple != "" {
		opts.Target = mustTarget(t, triple, fx.opts["sys"])
	}
	opts.Target.Allocator = fx.opts["allocator"]
	opts.Target.Profile = fx.opts["profile"]
	files := map[string]string{"mod.kg": "Module {\n    name: \"semdriver\"\n    kigumi: \"0.1\"\n}\n"}
	atRoot := fx.opts["stage"] == "driver" || fx.opts["mir"] == "planned"
	for name, body := range fx.files {
		if name == "main.kg" && !atRoot {
			name = "main/main.kg"
			opts.Entry = name
		}
		opts.Test = opts.Test || strings.HasSuffix(name, "_test.kg")
		files[name] = body
	}
	if e := fx.opts["entry"]; e != "" {
		opts.Entry = e
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, opts
}
