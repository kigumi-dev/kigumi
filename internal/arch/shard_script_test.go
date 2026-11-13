package arch_test

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// The driver package's shard selector must partition its tests: every test that slices its corpus
// through fixture.Shards runs in every shard, whole-corpus checks only in shard 0, the rest exactly once.
func TestShardScriptPartitionsDriverTests(t *testing.T) {
	root := "../.."
	pkg := "kigumi/internal/toolchain/driver"
	fixtureTests := shardCallers(t, root, "../toolchain/driver", pkg)
	wholeTests := []string{"TestRCCallCounts", "TestManifestFresh"}
	if len(fixtureTests) == 0 {
		t.Fatal("no driver test consults fixture.Shards; the call graph scan is broken")
	}
	for _, name := range wholeTests {
		if contains(fixtureTests, name) {
			t.Errorf("%s runs only in shard 0 but slices its corpus through fixture.Shards", name)
		}
	}
	t.Logf("corpus-slicing tests: %v", fixtureTests)

	existing := testNames(t, root, pkg)
	count := map[string]int{}
	for shard := range 4 {
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

// The AOT matrix must not repeat the VM corpus, and the hermetic job must not
// wait on native builds: each mode selects its own corpus tests in every shard
// and leaves the other job's tests out.
func TestShardScriptSplitsAOTAndVMCorpus(t *testing.T) {
	root := "../.."
	pkg := "kigumi/internal/toolchain/driver"
	aotTests := []string{"TestAotCorpus"}
	vmTests := []string{
		"TestArrayAcceleratorsMatchKigumiBodies",
		"TestConstFixturesEmitNatively",
		"TestCorpusOwnershipBeforeVerifier",
		"TestIRDump",
		"TestLazyStdBuildMatchesEager",
		"TestLazyStdCheckMatchesEager",
		"TestMutatedMIR",
		"TestRunCorpusEngines",
		"TestRunCorpusVMCoverage",
		"TestSemDriverFixtures",
		"TestStaticFixturesEmitNatively",
		"TestTextAcceleratorsMatchKigumiBodies",
		"TestUnitSweepSemFixtures",
	}
	fixtureTests := shardCallers(t, root, "../toolchain/driver", pkg)
	for _, name := range fixtureTests {
		if !contains(aotTests, name) && !contains(vmTests, name) {
			t.Errorf("the script classifies neither way the corpus test %s", name)
		}
	}
	for _, name := range append(append([]string{}, aotTests...), vmTests...) {
		if !contains(fixtureTests, name) {
			t.Errorf("the script calls %s a corpus test but it does not consult fixture.Shards", name)
		}
	}
	for shard := range 4 {
		aot := shardSelectionMode(t, root, shard, 4, pkg, "aot")
		for _, name := range aotTests {
			if !contains(aot, name) {
				t.Errorf("aot shard %d omits the native corpus test %s", shard, name)
			}
		}
		for _, name := range vmTests {
			if contains(aot, name) {
				t.Errorf("aot shard %d runs the no-C corpus test %s", shard, name)
			}
		}
		for _, name := range []string{"TestRCCallCounts", "TestManifestFresh"} {
			if contains(aot, name) {
				t.Errorf("aot shard %d runs the whole-corpus check %s", shard, name)
			}
		}
		vm := shardSelectionMode(t, root, shard, 4, pkg, "vm")
		for _, name := range vmTests {
			if !contains(vm, name) {
				t.Errorf("vm shard %d omits the corpus test %s", shard, name)
			}
		}
		for _, name := range aotTests {
			if contains(vm, name) {
				t.Errorf("vm shard %d runs the native corpus test %s", shard, name)
			}
		}
		for _, name := range []string{"TestRCCallCounts", "TestManifestFresh"} {
			if want := shard == 0; contains(vm, name) != want {
				t.Errorf("vm shard %d selects the whole-corpus check %s: want %v", shard, name, want)
			}
		}
	}

	cmd := exec.Command("bash", ".github/scripts/shard_tests.sh", "0", "4", pkg, "bogus")
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		t.Error("an unknown shard mode must fail")
	}
}

func shardSelectionMode(t *testing.T, root string, shard, count int, pkg, mode string) []string {
	t.Helper()
	cmd := exec.Command("bash", ".github/scripts/shard_tests.sh", strconv.Itoa(shard), strconv.Itoa(count), pkg, mode)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shard_tests.sh %d %s: %v", shard, mode, err)
	}
	expr := strings.TrimSpace(string(out))
	expr = strings.TrimSuffix(strings.TrimPrefix(expr, "^("), ")$")
	var names []string
	for _, name := range strings.Split(expr, "|") {
		if !contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}
