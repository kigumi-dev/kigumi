package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// The first draw waits for a second one to start, which then returns a different seed late.
const slowSeedC = `#include <stdatomic.h>
#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

static atomic_int kg_test_draws;

int getentropy(void* buf, size_t n) {
    int k = atomic_fetch_add(&kg_test_draws, 1);
    if (k == 0) {
        struct timespec ms = {0, 1000000};
        for (int i = 0; i < 3000 && atomic_load(&kg_test_draws) < 2; i++) nanosleep(&ms, 0);
    } else {
        static const char msg[] = "kg-test: second seed draw\n";
        write(2, msg, sizeof msg - 1);
        struct timespec later = {0, 300000000};
        nanosleep(&later, 0);
    }
    uint64_t v = 0x0101010101010101ULL * (uint64_t)(k + 1);
    memcpy(buf, &v, n < sizeof v ? n : sizeof v);
    return 0;
}
`

// Two workers race the first seed draw; a map filed under the losing draw must still find every key.
func TestMapSeedRaceKeepsOneSeed(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("the seed comes from getentropy on linux")
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	data, err := os.ReadFile("../../../testdata/run/map_thread_seed.txtar")
	if err != nil {
		t.Fatal(err)
	}
	src, want := txtarSection(string(data), "src"), txtarSection(string(data), "stdout")
	root := t.TempDir()
	shim := filepath.Join(t.TempDir(), "slow_seed.c")
	for path, body := range map[string]string{
		filepath.Join(root, "mod.kg"):  "Module {\n    name: \"mapseed\"\n    kigumi: \"0.1\"\n}\n",
		filepath.Join(root, "main.kg"): src,
		shim:                           slowSeedC,
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Target: buildconfig.HostTarget()})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{shim}}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	var out, errOut bytes.Buffer
	cmd := exec.Command(exe)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
	}
	if !strings.Contains(errOut.String(), "second seed draw") {
		t.Fatalf("the workers never raced the first draw, so the test proves nothing; stderr=%q", errOut.String())
	}
	if out.String() != want {
		t.Errorf("stdout=%q, want %q", out.String(), want)
	}
}

// txtarSection is the body of the section named name, up to the next section header.
func txtarSection(data, name string) string {
	_, body, _ := strings.Cut(data, "-- "+name+" --\n")
	if i := strings.Index(body, "\n-- "); i >= 0 {
		body = body[:i+1]
	}
	return body
}
