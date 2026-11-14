package arch_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The nightly chaos runner is one script that both CI and a local run execute, so the
// workflow's shard matrix cannot drift from what the script can run. These checks are
// hermetic: the script's plan/list/dry-run modes never invoke go test.
const chaosScript = ".github/scripts/chaos.sh"

type chaosEntry struct {
	Suite     string `json:"suite"`
	Shard     int    `json:"shard"`
	Count     int    `json:"count"`
	Seed      int64  `json:"seed"`
	Test      string `json:"test"`
	Toolchain string `json:"toolchain"`
	Seconds   int    `json:"seconds"`
	Mutants   int    `json:"mutants"`
	Timeout   int    `json:"timeout"`
}

func runChaos(t *testing.T, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("bash", append([]string{chaosScript}, args...)...)
	cmd.Dir = "../.."
	cmd.Env = append(envWithout("GITHUB_RUN_ID"), env...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("chaos.sh %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// envWithout keeps the caller's environment minus the named variables, so a seed test does
// not inherit a GITHUB_RUN_ID from the CI runner that launched it.
func envWithout(names ...string) []string {
	drop := map[string]bool{}
	for _, n := range names {
		drop[n] = true
	}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if !drop[k] {
			env = append(env, kv)
		}
	}
	return env
}

func chaosFails(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{chaosScript}, args...)...)
	cmd.Dir = "../.."
	if err := cmd.Run(); err == nil {
		t.Errorf("chaos.sh %s succeeded; want a hard failure", strings.Join(args, " "))
	}
}

// The plan must be a partition: every shard gets work, the corpus-slicing mir suite lands
// in every shard, and the two programs that ignore KIGUMI_TEST_SHARD run once, in shard 0.
// sem is opt-in (it is already gated on every push and costs more than a nightly budget), so
// the default plan must not carry it.
func TestChaosPlanPartitionsShards(t *testing.T) {
	const shards = 4
	out := runChaos(t, nil, "--list", "--shards", strconv.Itoa(shards), "--seed", "7")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "suite") {
		t.Fatalf("--list header = %q", out)
	}
	perShard := make([]int, shards)
	suiteShards := map[string][]int{}
	entries := map[[2]any]bool{}
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) != 5 {
			t.Fatalf("%q is not <suite> <shard> <count> <test> <toolchain>", line)
		}
		shard, err := strconv.Atoi(f[1])
		if err != nil {
			t.Fatalf("%q: shard is not a number", line)
		}
		count, err := strconv.Atoi(f[2])
		if err != nil || count != shards {
			t.Fatalf("%q: count is %s, want %d", line, f[2], shards)
		}
		if shard < 0 || shard >= shards {
			t.Fatalf("%q: shard out of range", line)
		}
		key := [2]any{f[0], shard}
		if entries[key] {
			t.Errorf("%s shard %d is listed twice", f[0], shard)
		}
		entries[key] = true
		perShard[shard]++
		suiteShards[f[0]] = append(suiteShards[f[0]], shard)
		if f[0] == "mir" || f[0] == "sem" {
			if f[4] != "go" {
				t.Errorf("%s must run without a C toolchain, toolchain = %s", f[0], f[4])
			}
		}
		if f[0] == "alloc-native" && f[4] != "zig" {
			t.Errorf("alloc-native needs zig, toolchain = %s", f[4])
		}
	}
	for shard, n := range perShard {
		if n == 0 {
			t.Errorf("shard %d/%d is empty; every shard must run real tests", shard, shards)
		}
	}
	if got := suiteShards["mir"]; len(got) != shards {
		t.Errorf("mir runs in %d shards, want %d", len(got), shards)
	}
	for _, suite := range []string{"alloc", "alloc-native"} {
		if got := suiteShards[suite]; len(got) != 1 || got[0] != 0 {
			t.Errorf("%s runs in shards %v, want once in shard 0", suite, got)
		}
	}
	for _, suite := range []string{"mir", "alloc", "alloc-native"} {
		if len(suiteShards[suite]) == 0 {
			t.Errorf("suite %s disappeared from the plan", suite)
		}
	}
	if got := suiteShards["sem"]; len(got) != 0 {
		t.Errorf("sem runs nightly in %d shards; it is opt-in (--suite sem)", len(got))
	}

	// An explicit --suite sem still plans one slice per shard, so the opt-in path stays
	// runnable and remains the replay route for the push-gated sweep.
	var sem []chaosEntry
	if err := json.Unmarshal([]byte(runChaos(t, nil, "--plan", "--suite", "sem", "--shards", strconv.Itoa(shards), "--seed", "7")), &sem); err != nil {
		t.Fatalf("--suite sem --plan: %v", err)
	}
	if len(sem) != shards {
		t.Fatalf("--suite sem planned %d entries, want %d", len(sem), shards)
	}
	for i, e := range sem {
		if e.Suite != "sem" || e.Shard != i || e.Test != "TestUnitSweepSemFixtures" || e.Toolchain != "go" {
			t.Errorf("sem plan entry %d = %+v", i, e)
		}
	}
}

// Every planned test name must still exist in the driver package: the plan is a contract
// with the test sources, not a copy of their names.
func TestChaosPlanNamesLiveTests(t *testing.T) {
	out := runChaos(t, nil, "--list", "--shards", "2", "--seed", "7")
	driveDir := filepath.Join("..", "toolchain", "driver")
	files, err := filepath.Glob(filepath.Join(driveDir, "*_test.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("driver test sources: %v", err)
	}
	var sources strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		sources.Write(b)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) != 5 {
			continue
		}
		if !strings.Contains(sources.String(), "func "+f[3]+"(") {
			t.Errorf("plan names %s, which internal/toolchain/driver does not define", f[3])
		}
	}
}

// A shard's seed is a pure function of (base seed, suite, shard): the same run twice prints
// the same plan, and an explicit seed is what makes a red night replayable.
func TestChaosPlanSeedsAreDerivedAndDistinct(t *testing.T) {
	first := runChaos(t, nil, "--plan", "--shards", "4", "--seed", "7", "--seconds", "30", "--count", "40")
	second := runChaos(t, nil, "--plan", "--shards", "4", "--seed", "7", "--seconds", "30", "--count", "40")
	if first != second {
		t.Fatalf("the same base seed printed two plans:\n%s\n%s", first, second)
	}
	var entries []chaosEntry
	if err := json.Unmarshal([]byte(first), &entries); err != nil {
		t.Fatalf("--plan is not a JSON array: %v\n%s", err, first)
	}
	if len(entries) == 0 {
		t.Fatal("--plan printed no entries")
	}
	seen := map[int64]string{}
	for _, e := range entries {
		if e.Test == "" || e.Toolchain == "" {
			t.Errorf("entry %+v misses test or toolchain", e)
		}
		if e.Seconds != 30 || e.Mutants != 40 || e.Timeout <= 0 {
			t.Errorf("entry %+v did not carry the requested knobs", e)
		}
		if prev, ok := seen[e.Seed]; ok {
			t.Errorf("%s shard %d and %s share seed %d", e.Suite, e.Shard, prev, e.Seed)
		}
		seen[e.Seed] = e.Suite + "#" + strconv.Itoa(e.Shard)
	}
	other := runChaos(t, nil, "--plan", "--shards", "4", "--seed", "8")
	var otherEntries []chaosEntry
	if err := json.Unmarshal([]byte(other), &otherEntries); err != nil {
		t.Fatalf("--plan with seed 8: %v", err)
	}
	same := 0
	for i := range entries {
		if i < len(otherEntries) && entries[i].Seed == otherEntries[i].Seed {
			same++
		}
	}
	if same == len(entries) {
		t.Error("changing the base seed left every shard seed unchanged")
	}

	if got := strings.TrimSpace(runChaos(t, nil, "--print-seed", "--seed", "7")); got != "7" {
		t.Errorf("--print-seed --seed 7 = %q", got)
	}
	runEnv := []string{"GITHUB_RUN_ID=1234"}
	a := strings.TrimSpace(runChaos(t, runEnv, "--print-seed"))
	b := strings.TrimSpace(runChaos(t, runEnv, "--print-seed"))
	if a != b {
		t.Errorf("one run id printed seeds %q and %q", a, b)
	}
	c := strings.TrimSpace(runChaos(t, []string{"GITHUB_RUN_ID=1235"}, "--print-seed"))
	if a == c {
		t.Errorf("different run ids both printed seed %q", a)
	}
}

// --dry-run must show the command the real run would execute, per entry, with the shard
// selector and the mir seed that a log would carry.
func TestChaosDryRunMatchesPlan(t *testing.T) {
	const shards = 4
	var entries []chaosEntry
	if err := json.Unmarshal([]byte(runChaos(t, nil, "--plan", "--shards", strconv.Itoa(shards), "--seed", "7")), &entries); err != nil {
		t.Fatal(err)
	}
	out := runChaos(t, nil, "--dry-run", "--shards", strconv.Itoa(shards), "--seed", "7")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != len(entries) {
		t.Fatalf("--dry-run printed %d commands for %d plan entries", len(lines), len(entries))
	}
	for i, line := range lines {
		e := entries[i]
		if !strings.Contains(line, "go test") || !strings.Contains(line, "-run '^"+e.Test+"$'") {
			t.Errorf("dry-run line %q does not run %s", line, e.Test)
		}
		if !strings.Contains(line, "-timeout ") {
			t.Errorf("dry-run line %q has no bounded timeout", line)
		}
		if !strings.Contains(line, "KIGUMI_TEST_SHARD="+strconv.Itoa(e.Shard)+"/"+strconv.Itoa(shards)) {
			t.Errorf("dry-run line %q lacks the shard selector", line)
		}
		seed := "KIGUMI_MIR_FUZZ_FROM=" + strconv.FormatInt(e.Seed, 10)
		if e.Suite == "mir" {
			if !strings.Contains(line, seed) || !strings.Contains(line, "KIGUMI_MIR_FUZZ_COUNT=") {
				t.Errorf("mir dry-run line %q lacks the seed/count knobs", line)
			}
		} else if strings.Contains(line, "KIGUMI_MIR_FUZZ_FROM=") {
			t.Errorf("%s dry-run line %q carries a mir-only knob", e.Suite, line)
		}
	}

	filtered := strings.TrimSpace(runChaos(t, nil, "--dry-run", "--suite", "alloc-native", "--shard", "0", "--shards", "4"))
	if strings.Count(filtered, "\n") != 0 || !strings.Contains(filtered, "TestFaultInjectionSweepNative") {
		t.Errorf("--suite alloc-native --shard 0 printed %q", filtered)
	}
}

func TestChaosRejectsBadSelectors(t *testing.T) {
	for _, args := range [][]string{
		{"--shards", "0"},
		{"--shards", "x"},
		{"--shard", "4", "--shards", "4"},
		{"--suite", "nope"},
		{"--suite", "alloc", "--shard", "1", "--shards", "4"},
		{"--bogus"},
	} {
		chaosFails(t, args...)
	}
}

// The workflow is a thin caller: it resolves the seed, asks the script for the matrix, and
// uploads the logs. It must keep the toolchain pins the other workflows use and must not
// duplicate nightly-fuzz.yml's coverage-guided fuzzing.
func TestChaosWorkflowContract(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "nightly-chaos.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(workflow)
	for _, want := range []string{
		"schedule:",
		"cron:",
		"workflow_dispatch:",
		"seed:",
		"seconds:",
		"shards:",
		"fail-fast: false",
		"fromJSON(needs.plan.outputs.matrix)",
		chaosScript,
		"--print-seed",
		"--plan",
		"upload-artifact",
		"if: always()",
		"timeout-minutes:",
		"needs: [plan, zig-version]",
		"go run . zig install ${{ needs.zig-version.outputs.version }}",
		"kigumi-zig-${{ runner.os }}-${{ runner.arch }}-${{ needs.zig-version.outputs.version }}",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("nightly-chaos.yml no longer contains %q", want)
		}
	}
	if regexp.MustCompile(`(?m)^\s*- run:.*-fuzz`).MatchString(text) || strings.Contains(text, "-fuzztime") {
		t.Error("nightly-chaos.yml grew a -fuzz step; nightly-fuzz.yml owns coverage-guided fuzzing")
	}
	if strings.Contains(text, "suite: mir") {
		t.Error("nightly-chaos.yml hardcodes a shard matrix; the script's --plan must own it")
	}

	// Go and Zig versions are pinned once; the chaos workflow must not drift from them.
	goMod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	goVersion := regexp.MustCompile(`(?m)^go (\S+)`).FindStringSubmatch(string(goMod))
	if goVersion == nil {
		t.Fatal("go.mod has no go directive")
	}
	if !strings.Contains(text, `go-version: "`+goVersion[1]+`"`) {
		t.Errorf("nightly-chaos.yml does not pin go-version %q from go.mod", goVersion[1])
	}
	if !strings.Contains(text, "cat ZIG_VERSION") {
		t.Error("nightly-chaos.yml must read the pinned ZIG_VERSION")
	}

	// The plan job is the only place the matrix originates, so the script's zig entries are
	// what install zig; assert the workflow keys its zig steps off that field.
	if !strings.Contains(text, "matrix.toolchain == 'zig'") {
		t.Error("nightly-chaos.yml does not gate the zig install on the plan's toolchain field")
	}

	fuzz, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "nightly-fuzz.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fuzz), "-fuzz") {
		t.Error("nightly-fuzz.yml stopped fuzzing; the ownership split assumed it keeps -fuzz")
	}
}
