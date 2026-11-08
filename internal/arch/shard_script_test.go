package arch_test

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// The driver package's shard selector must partition its tests: every test that slices its corpus
// through fixture.Shard runs in every shard, whole-corpus checks only in shard 0, the rest exactly once.
func TestShardScriptPartitionsDriverTests(t *testing.T) {
	root := "../.."
	pkg := "kigumi/internal/toolchain/driver"
	fixtureTests := shardCallers(t, root, "../toolchain/driver", pkg)
	wholeTests := []string{"TestRCCallCounts", "TestManifestFresh"}
	if len(fixtureTests) == 0 {
		t.Fatal("no driver test consults fixture.Shard; the call graph scan is broken")
	}
	for _, name := range wholeTests {
		if contains(fixtureTests, name) {
			t.Errorf("%s runs only in shard 0 but slices its corpus through fixture.Shard", name)
		}
	}
	t.Logf("corpus-slicing tests: %v", fixtureTests)

	existing := testNames(t, root, pkg)
	count := map[string]int{}
	for shard := 0; shard < 4; shard++ {
		got := shardSelection(t, root, shard, pkg)
		for _, name := range got {
			if !existing[name] {
				t.Errorf("shard %d selects %s, which is not a test in %s", shard, name, pkg)
			}
			count[name]++
		}
		for _, name := range fixtureTests {
			if !contains(got, name) {
				t.Errorf("shard %d omits the corpus test %s", shard, name)
			}
		}
		for _, name := range wholeTests {
			if want := shard == 0; contains(got, name) != want {
				t.Errorf("shard %d selects %s: want %v", shard, name, want)
			}
		}
	}
	for _, name := range fixtureTests {
		if count[name] != 4 {
			t.Errorf("%s runs in %d shards, want 4", name, count[name])
		}
	}
	for _, name := range wholeTests {
		if count[name] != 1 {
			t.Errorf("%s runs in %d shards, want 1", name, count[name])
		}
	}
	for name := range existing {
		if contains(fixtureTests, name) || contains(wholeTests, name) {
			continue
		}
		if count[name] != 1 {
			t.Errorf("%s runs in %d shards, want 1", name, count[name])
		}
	}

	cmd := exec.Command("bash", ".github/scripts/shard_tests.sh", "0", "4", "kigumi/does-not-exist")
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		t.Error("a package go test cannot list must fail the selector")
	}
}

func testNames(t *testing.T, root, pkg string) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "test", "-list", "^Test", pkg)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -list %s: %v\n%s", pkg, err, out)
	}
	names := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Test") {
			names[line] = true
		}
	}
	if len(names) == 0 {
		t.Fatalf("go test -list %s returned no tests", pkg)
	}
	return names
}

func shardSelection(t *testing.T, root string, shard int, pkg string) []string {
	t.Helper()
	cmd := exec.Command("bash", ".github/scripts/shard_tests.sh", strconv.Itoa(shard), "4", pkg)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shard_tests.sh %d: %v", shard, err)
	}
	expr := strings.TrimSpace(string(out))
	expr = strings.TrimSuffix(strings.TrimPrefix(expr, "^("), ")$")
	// -run alternatives are a set: a repeated name is harmless after `go test`
	// compiles it, and deduplicating keeps the partition assertion about
	// membership rather than a spelling detail of the generated regex.
	var names []string
	seen := map[string]bool{}
	for _, name := range strings.Split(expr, "|") {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
