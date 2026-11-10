package shell

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/compiler/syntax"
)

// TestRunHereInheritsStreams pins the foreground runner: stage 0 reads the
// given stdin, the last stage writes the given stdout, all stages share the
// given stderr, and nothing is captured into the statuses.
func TestRunHereInheritsStreams(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX printf and sh")
	}
	cwd := t.TempDir()
	var out, errOut bytes.Buffer
	statuses, err := RunHere(&Plan{Stages: []Stage{{Argv: []string{"cat"}}}}, cwd, strings.NewReader("in"), &out, &errOut)
	if err != nil {
		t.Fatalf("cat: %v", err)
	}
	if out.String() != "in" || errOut.String() != "" {
		t.Fatalf("stdout = %q, stderr = %q", out.String(), errOut.String())
	}
	if len(statuses) != 1 || statuses[0] != 0 {
		t.Fatalf("statuses = %v", statuses)
	}

	out.Reset()
	statuses, err = RunHere(&Plan{Stages: []Stage{{Argv: []string{"sh", "-c", "exit 3"}}}}, cwd, nil, &out, &errOut)
	if err != nil {
		t.Fatalf("exit 3: %v", err)
	}
	if len(statuses) != 1 || statuses[0] != 3 {
		t.Fatalf("exit status = %v, want [3]", statuses)
	}
}

// TestRunHerePipelineAndFilters covers stage chaining and an in-process
// filter writing the final stdout directly.
func TestRunHerePipelineAndFilters(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX printf")
	}
	cwd := t.TempDir()
	var out, errOut bytes.Buffer
	plan := &Plan{Stages: []Stage{
		{Argv: []string{"printf", "a\nb\n"}},
		{Filter: GrepFilter("a")},
	}}
	statuses, err := RunHere(plan, cwd, nil, &out, &errOut)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if out.String() != "a\n" {
		t.Fatalf("stdout = %q", out.String())
	}
	if len(statuses) != 2 || statuses[0] != 0 || statuses[1] != 0 {
		t.Fatalf("statuses = %v", statuses)
	}
}

// TestRunHereStartupFailure: a command or redirect that cannot start is an
// error, not a fake exit status, and the stages already started are reaped.
func TestRunHereStartupFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX printf")
	}
	cwd := t.TempDir()
	var out, errOut bytes.Buffer
	if _, err := RunHere(&Plan{Stages: []Stage{{Argv: []string{"/nonexistent-kigumi-run-here"}}}}, cwd, nil, &out, &errOut); err == nil {
		t.Fatal("a missing program must fail")
	}
	plan := &Plan{Stages: []Stage{{
		Argv:   []string{"printf", "x"},
		Redirs: []Redirect{{Op: syntax.RedirOut, Fd: 1, Target: "/nonexistent-kigumi-run-here-dir/out"}},
	}}}
	if _, err := RunHere(plan, cwd, nil, &out, &errOut); err == nil {
		t.Fatal("an unopenable redirect must fail")
	}
}

// TestRunHereConcurrentStderr: each external stage's stderr may be copied by
// its own goroutine, so a shared buffer must still be written safely.
func TestRunHereConcurrentStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sh")
	}
	cwd := t.TempDir()
	var out, errOut bytes.Buffer
	plan := &Plan{Stages: []Stage{
		{Argv: []string{"sh", "-c", "printf a >&2"}},
		{Argv: []string{"sh", "-c", "printf b >&2"}},
	}}
	if _, err := RunHere(plan, cwd, nil, &out, &errOut); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if got := errOut.String(); !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Fatalf("stderr = %q, want both stages' output", got)
	}
}
