package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

const oomDir = "../../testdata/oom"

// The fixtures eprint once after their setup, which arms the tick counter, so a refused tick lands inside the call and nowhere else.
var tryFamilyWindow = regexp.MustCompile(`^E+O{3,}$`)

const tryFamilySweep = 160

func oomSources(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(oomDir, "*.kg"))
	if err != nil || len(files) < 20 {
		t.Fatalf("found %d programs in %s: %v", len(files), oomDir, err)
	}
	out := map[string]string{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimSuffix(filepath.Base(f), ".kg")] = string(src)
	}
	return out
}

// classifyTryRun is one letter per run: O success, E returned Err with the receiver intact, P a panic, C receiver changed, ? anything else.
func classifyTryRun(exit int, stdout string) byte {
	switch {
	case exit == 0 && stdout == "":
		return 'O'
	case exit == 0 && stdout == "err\n":
		return 'E'
	case exit == 0 && strings.HasPrefix(stdout, "CHANGED"):
		return 'C'
	case exit == 2:
		return 'P'
	}
	return '?'
}

var liveLine = regexp.MustCompile(`(?m)^kigumi-live: (\d+)$`)

func TestFaultInjectionTryFamilyNative(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for name, src := range oomSources(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			exe := filepath.Join(t.TempDir(), "prog")
			var buildErr bytes.Buffer
			ok, err := driver.BuildWith(loadTryConstructorModule(t, src), exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT", "-DKIGUMI_RC_CHECK"}})
			if err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, buildErr.String())
			}
			run := func(failAt int, report bool) (byte, int64) {
				cmd := exec.Command(exe)
				cmd.Dir = t.TempDir()
				cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_ARM=1")
				if report {
					cmd.Env = append(cmd.Env, "KIGUMI_LIVE_REPORT=1")
				}
				if failAt > 0 {
					cmd.Env = append(cmd.Env, "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(failAt))
				}
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				exit := exitCodeOf(cmd.Run())
				live := int64(-1)
				if m := liveLine.FindStringSubmatch(stderr.String()); m != nil {
					live, _ = strconv.ParseInt(m[1], 10, 64)
				}
				return classifyTryRun(exit, stdout.String()), live
			}
			base, baseLive := run(0, true)
			if base != 'O' || baseLive < 0 {
				t.Fatalf("the unfaulted run is %c with live count %d, want a silent success with a live count", base, baseLive)
			}
			// The live report allocates after the program ends, so classifying runs go without it and each Err run is rerun with it.
			var classes []byte
			for n := 1; n <= tryFamilySweep; n++ {
				c, _ := run(n, false)
				classes = append(classes, c)
				if c != 'E' {
					continue
				}
				if _, live := run(n, true); live != baseLive {
					t.Errorf("N=%d: %d objects live after the Err return, %d without a fault", n, live, baseLive)
				}
			}
			checkTryFamilyWindow(t, name, classes, true)
		})
	}
}

// tryFamilyNoAlloc are the fixtures on a path that allocates nothing, so no tick can be refused inside the call.
var tryFamilyNoAlloc = map[string]bool{"map_set_existing_key": true, "map_set_existing_key_twice": true, "alloc_wrap": true, "alloc_wrap_twice": true}

// tryFamilyNoAllocNative build their Result inline and copy or wrap nothing that allocates; the VM still ticks once per try op.
var tryFamilyNoAllocNative = map[string]bool{"shared_get": true, "shared_get_twice": true}

var tryFamilyNoAllocWindow = regexp.MustCompile(`^O+$`)

func checkTryFamilyWindow(t *testing.T, name string, classes []byte, native bool) {
	t.Helper()
	window := tryFamilyWindow
	if tryFamilyNoAlloc[name] || native && tryFamilyNoAllocNative[name] {
		window = tryFamilyNoAllocWindow
	}
	if !window.Match(classes) {
		t.Errorf("%s: fault sweep outcomes %s do not match %s", name, classes, window)
	}
}

func TestFaultInjectionTryFamilyVM(t *testing.T) {
	t.Parallel()
	for name, src := range oomSources(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, err := driver.Analyze(loadTryConstructorModule(t, src))
			if err != nil || a.Res.HasErrors() {
				t.Fatalf("analyze: %v\n%s", err, a.Res.Render())
			}
			var classes []byte
			for n := 1; n <= tryFamilySweep/2; n++ {
				var out, errOut bytes.Buffer
				code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n), FaultArmOnEprint: true}, &out, &errOut)
				if err != nil {
					t.Fatalf("N=%d: %v", n, err)
				}
				classes = append(classes, classifyTryRun(code, out.String()))
			}
			checkTryFamilyWindow(t, name, classes, false)
		})
	}
}
