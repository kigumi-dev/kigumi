package arch_test

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// The driver package's shard selector must partition its tests: corpus loops run in every shard
// (each takes its fixture slice), whole-corpus checks only in shard 0, everything else exactly once.
func TestShardScriptPartitionsDriverTests(t *testing.T) {
	root := "../.."
	pkg := "kigumi/internal/toolchain/driver"
	fixtureTests := []string{"TestAotCorpus", "TestRunCorpusEngines", "TestSemDriverFixtures", "TestTextAcceleratorsMatchKigumiBodies", "TestArrayAcceleratorsMatchKigumiBodies"}
	wholeTests := []string{"TestRCCallCounts", "TestCorpusOwnershipBeforeVerifier", "TestManifestFresh"}

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
	return strings.Split(expr, "|")
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
