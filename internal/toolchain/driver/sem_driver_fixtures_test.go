package driver_test

import (
	"kigumi/internal/devkit/fixture"
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

type semDriverFixture struct {
	opts    map[string]string
	files   map[string]string
	expect  string
	expectP int
	raw     string
}

func parseSemDriverFixture(t *testing.T, path string) semDriverFixture {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fx := semDriverFixture{opts: map[string]string{}, files: map[string]string{}, raw: string(data), expectP: -1}
	name, body, off := "", "", 0
	flush := func() {
		switch name {
		case "":
		case "module":
			for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
				k, v, _ := strings.Cut(line, ":")
				fx.opts[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		case "src":
			fx.files["main.kg"] = body
		default:
			fx.files[name] = body
		}
	}
	for _, line := range strings.SplitAfter(fx.raw, "\n") {
		trimmed := strings.TrimRight(line, "\n")
		off += len(line)
		if strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") {
			flush()
			name, body = strings.TrimSuffix(strings.TrimPrefix(trimmed, "-- "), " --"), ""
			if name == "expect" {
				fx.expectP, fx.expect = off, fx.raw[off:]
				return fx
			}
			continue
		}
		body += line
	}
	t.Fatalf("%s: missing -- expect -- section", path)
	return fx
}

func TestSemDriverFixtures(t *testing.T) {
	paths, _ := filepath.Glob("../../../testdata/sem/*.txtar")
	if len(paths) == 0 {
		t.Fatal("no sem fixtures")
	}
	ran := 0
	for _, path := range paths {
		if !fixture.Shard(filepath.Base(path)) {
			continue
		}
		fx := parseSemDriverFixture(t, path)
		if fx.opts["stage"] != "driver" {
			continue
		}
		ran++
		t.Run(strings.TrimSuffix(filepath.Base(path), ".txtar"), func(t *testing.T) {
			t.Parallel()
			target := buildconfig.HostTarget()
			if triple := fx.opts["triple"]; triple != "" {
				target = mustTarget(t, triple, fx.opts["sys"])
			}
			target.Allocator = fx.opts["allocator"]
			target.Profile = fx.opts["profile"]
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"semdriver\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			for name, body := range fx.files {
				os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755)
				os.WriteFile(filepath.Join(root, name), []byte(body), 0o644)
			}
			std, _ := filepath.Abs("../../../std")
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
			if err != nil {
				t.Fatal(err)
			}
			res, err := driver.Check(m)
			if err != nil {
				t.Fatal(err)
			}
			got := res.Render()
			if os.Getenv("UPDATE") == "1" {
				if err := os.WriteFile(path, []byte(fx.raw[:fx.expectP]+got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if got != fx.expect {
				t.Errorf("expect mismatch (UPDATE=1 to accept)\n--- got ---\n%s--- want ---\n%s", got, fx.expect)
			}
		})
	}
	if ran == 0 {
		t.Fatal("no testdata/sem fixture is marked `stage: driver`")
	}
}
