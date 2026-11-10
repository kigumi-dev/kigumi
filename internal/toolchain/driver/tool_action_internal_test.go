package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/artifact"
	"kigumi/internal/toolchain/buildgraph"
)

// toolRun is one graph with a single Framework tool step whose script copies
// its declared input to its declared output and records every run, so a test
// can tell a cache hit from a rerun and prove which tool version produced the
// bytes on disk.
type toolRun struct {
	outDir  string
	input   string
	counter string
	output  string
	script  string
	rg      *ResolvedGraph
	paths   map[int]string
	cache   *actionCache
	stats   actionCacheStats
	opts    ExecOptions
	stderr  bytes.Buffer
}

func toolScript(marker string) string {
	return "#!/bin/sh\nprintf '%s' '" + marker + "' > \"$1\"\ncat \"$3\" >> \"$1\"\necho run >> \"$2\"\n"
}

func newToolRun(t *testing.T, version string) *toolRun {
	t.Helper()
	r := &toolRun{
		outDir:  t.TempDir(),
		input:   filepath.Join(t.TempDir(), "src.txt"),
		counter: filepath.Join(t.TempDir(), "runs.log"),
		script:  filepath.Join(t.TempDir(), "tool.sh"),
	}
	r.output = filepath.Join(r.outDir, "copy.txt")
	r.paths = map[int]string{0: r.input, 1: r.output}
	r.writeInput(t, "one\n")
	r.writeScript(t, "")
	if err := os.Chmod(r.script, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &buildgraph.Graph{
		Roots:      []string{r.outDir},
		Frameworks: []buildgraph.FrameworkRef{{Name: "fw", Version: version}},
		Files: []buildgraph.FileRef{
			{ID: 0, Kind: "literal", Path: filepath.Base(r.input)},
			{ID: 1, Kind: "toolOutput", Path: "copy.txt", ProducedBy: 0},
		},
		ToolSteps: []buildgraph.ToolStep{{
			ID:        0,
			Framework: 0,
			Tool:      "copy",
			Args:      []string{"$out:0", r.counter, "$in:0"},
			Inputs:    []int{0},
			Outputs:   []int{1},
		}},
	}
	r.rg = &ResolvedGraph{
		Graph: g,
		Frameworks: []FrameworkDescriptor{{
			Name:    "fw",
			Version: version,
			Tools:   map[string]ToolProgram{"copy": {Path: r.script}},
		}},
	}
	r.cache = newActionCache(filepath.Join(r.outDir, actionCacheDirName))
	r.opts = ExecOptions{ToolEnv: []string{"PATH=" + os.Getenv("PATH")}}
	return r
}

func (r *toolRun) writeInput(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(r.input, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (r *toolRun) writeScript(t *testing.T, marker string) {
	t.Helper()
	if err := os.WriteFile(r.script, []byte(toolScript(marker)), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (r *toolRun) filePath(id int) string { return r.paths[id] }

// run is one build: each call gets its own cache object, exactly as a new
// ExecuteGraph does, and only the recorded entries and the fixture survive.
func (r *toolRun) run(t *testing.T) error {
	t.Helper()
	r.stderr.Reset()
	r.cache = newActionCache(filepath.Join(r.outDir, actionCacheDirName))
	err := runToolStep(r.rg, 0, r.filePath, r.opts, r.outDir, r.cache, &r.stderr)
	r.stats.Hits += r.cache.stats.Hits
	r.stats.Misses += r.cache.stats.Misses
	r.stats.Stores += r.cache.stats.Stores
	r.stats.Uncacheable += r.cache.stats.Uncacheable
	return err
}

// runs counts how many times the tool process actually executed.
func (r *toolRun) runs(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile(r.counter)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "run\n")
}

func (r *toolRun) readOutput(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(r.output)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// action is the identity the step currently hashes to, computed the way
// runToolStep computes it.
func (r *toolRun) action(t *testing.T) artifact.Action {
	t.Helper()
	s := r.rg.Graph.ToolSteps[0]
	tp, _ := r.rg.Frameworks[0].program(s.Tool)
	env := artifact.CanonicalEnv(toolEnv(r.opts.ToolEnv, r.rg.Graph.Request.Target))
	a, ok := toolAction(r.cache, r.rg, s, tp.Path, s.Args, env, r.filePath, []string{r.rg.Graph.Files[1].Path})
	if !ok {
		t.Fatal("the fixture action must have a complete identity")
	}
	return a
}

// TestToolActionReusesIdenticalAction covers the cache contract: an unchanged
// tool, argv, environment and input set restores the recorded outputs instead
// of running the tool again.
func TestToolActionReusesIdenticalAction(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	if err := r.run(t); err != nil {
		t.Fatalf("first run: %v\n%s", err, r.stderr.String())
	}
	if got := r.readOutput(t); got != "one\n" {
		t.Fatalf("first output = %q", got)
	}
	if r.runs(t) != 1 || r.stats.Misses != 1 || r.stats.Stores != 1 {
		t.Fatalf("first run: %d runs, stats %+v", r.runs(t), r.cache.stats)
	}
	if err := os.Remove(r.output); err != nil {
		t.Fatal(err)
	}
	if err := r.run(t); err != nil {
		t.Fatalf("second run: %v\n%s", err, r.stderr.String())
	}
	if r.runs(t) != 1 {
		t.Fatalf("an identical action ran again (%d runs)", r.runs(t))
	}
	if r.stats.Hits != 1 {
		t.Fatalf("second run stats = %+v, want one hit", r.cache.stats)
	}
	if !strings.Contains(r.stderr.String(), "reuses cached outputs") {
		t.Errorf("a hit should be reported: %s", r.stderr.String())
	}
	if got := r.readOutput(t); got != "one\n" {
		t.Fatalf("restored output = %q, want the recorded bytes", got)
	}
}

// TestToolActionSplitsOnInputChange rewrites a declared input: the old action
// must not be reused, and the output must be the new tool result.
func TestToolActionSplitsOnInputChange(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	r.writeInput(t, "two\n")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	if r.runs(t) != 2 || r.stats.Misses != 2 {
		t.Fatalf("a changed input reused the old action: %d runs, %+v", r.runs(t), r.cache.stats)
	}
	if got := r.readOutput(t); got != "two\n" {
		t.Fatalf("output = %q, want the new input", got)
	}
}

// TestToolActionSplitsOnEnv changes the child environment: PATH is part of
// what the tool can observe, so the action must split.
func TestToolActionSplitsOnEnv(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	r.opts.ToolEnv = []string{"PATH=" + os.Getenv("PATH"), "KIGUMI_EXTRA=1"}
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	if r.runs(t) != 2 {
		t.Fatalf("a changed environment reused the action (%d runs)", r.runs(t))
	}
	// The same variables in another order are the same environment.
	r.opts.ToolEnv = []string{"KIGUMI_EXTRA=1", "PATH=" + os.Getenv("PATH")}
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	if r.runs(t) != 2 {
		t.Fatalf("a reordered environment split the action (%d runs)", r.runs(t))
	}
}

// TestToolActionSplitsOnArgv adds an argument: the invocation differs even
// though the tool and inputs are the same.
func TestToolActionSplitsOnArgv(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	r.rg.Graph.ToolSteps[0].Args = append(r.rg.Graph.ToolSteps[0].Args, "--verbose")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	if r.runs(t) != 2 {
		t.Fatalf("a changed argv reused the action (%d runs)", r.runs(t))
	}
}

// TestToolActionSplitsOnToolVersion is the negative test for a tool version
// change: the executable bytes and every other component stay identical, so
// only the Framework's declared version tells the two tools apart.
func TestToolActionSplitsOnToolVersion(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	before := r.action(t)
	r.rg.Graph.Frameworks[0].Version = "2"
	r.rg.Frameworks[0].Version = "2"
	after := r.action(t)
	if before.Key() == after.Key() {
		t.Fatal("a tool version change kept the action key")
	}
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	if r.runs(t) != 2 {
		t.Fatalf("a tool version change reused the recorded action (%d runs)", r.runs(t))
	}
	if got := r.readOutput(t); got != "one\n" {
		t.Fatalf("output = %q", got)
	}
}

// TestToolActionSplitsOnToolDigest rewrites the tool executable: the recorded
// output came from other bytes, so the step must run again and its output
// must be the new tool's, never the stale one.
func TestToolActionSplitsOnToolDigest(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	r.writeScript(t, "v2:")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	if r.runs(t) != 2 {
		t.Fatalf("a rewritten tool reused the action (%d runs)", r.runs(t))
	}
	if got := r.readOutput(t); got != "v2:one\n" {
		t.Fatalf("output = %q, want the new tool's bytes", got)
	}
}

// TestToolActionSplitsOnOutputName covers a declared output rename: the
// recorded entry belongs to other output slots.
func TestToolActionSplitsOnOutputName(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	r.rg.Graph.Files[1].Path = "renamed.txt"
	r.paths[1] = filepath.Join(r.outDir, "renamed.txt")
	if err := r.run(t); err != nil {
		t.Fatal(err)
	}
	if r.runs(t) != 2 {
		t.Fatalf("a renamed output reused the action (%d runs)", r.runs(t))
	}
}

// TestToolActionSkipsSecrets pins that a secret-taking step is never cached:
// the secret's bytes are not in the key, so reusing its result would be a
// stale artifact.
func TestToolActionSkipsSecrets(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	secret := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(secret, []byte("s3cret"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.rg.Graph.Secrets = []buildgraph.SecretRef{{Name: "apikey"}}
	r.rg.Graph.ToolSteps[0].Secrets = []int{0}
	r.opts.Secrets = map[string]string{"apikey": secret}
	for i := range 2 {
		if err := os.RemoveAll(r.output); err != nil {
			t.Fatal(err)
		}
		if err := r.run(t); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
		if got := r.readOutput(t); got != "one\n" {
			t.Fatalf("run %d output = %q", i+1, got)
		}
	}
	if r.runs(t) != 2 || r.stats.Uncacheable != 2 || r.stats.Hits != 0 {
		t.Fatalf("secret steps must always run: %d runs, %+v", r.runs(t), r.cache.stats)
	}
}

// TestToolActionUnavailableIdentityRuns checks that a step whose executable
// cannot be resolved is not cached, rather than keyed from partial identity.
func TestToolActionUnavailableIdentityRuns(t *testing.T) {
	t.Parallel()
	r := newToolRun(t, "1")
	r.rg.Frameworks[0].Tools["copy"] = ToolProgram{Path: filepath.Join(t.TempDir(), "missing")}
	if err := r.run(t); err == nil {
		t.Fatal("a missing tool executable must fail the step")
	}
	if r.stats.Uncacheable != 1 {
		t.Fatalf("stats = %+v, want one uncacheable step", r.cache.stats)
	}
}
