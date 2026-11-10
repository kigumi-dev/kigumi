package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/buildgraph"
)

// TestRunToolStepKeepsStaleOutputOnNoOpSuccess checks that a tool exiting 0 without writing a declared output is refused, not hidden by a previous build's leftover file.
func TestRunToolStepKeepsStaleOutputOnNoOpSuccess(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	stale := filepath.Join(outDir, "gen.txt")
	if err := os.WriteFile(stale, []byte("STALE"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &buildgraph.Graph{
		Roots: []string{outDir},
		Files: []buildgraph.FileRef{{ID: 0, Kind: "toolOutput", Path: "gen.txt", ProducedBy: 0}},
		ToolSteps: []buildgraph.ToolStep{
			{ID: 0, Framework: 0, Tool: "noop", Outputs: []int{0}},
		},
	}
	rg := &ResolvedGraph{
		Graph:      g,
		Frameworks: []FrameworkDescriptor{{Name: "fw", Tools: map[string]ToolProgram{"noop": {Path: "/bin/true"}}}},
	}
	filePath := func(id int) string { return filepath.Join(outDir, g.Files[id].Path) }
	var stderr bytes.Buffer
	err := runToolStep(rg, 0, filePath, ExecOptions{}, outDir, nil, &stderr)
	if err == nil {
		t.Fatal("a tool that writes nothing must be refused, not silently satisfied by a stale file")
	}
	got, readErr := os.ReadFile(stale)
	if readErr != nil || string(got) != "STALE" {
		t.Fatalf("the stale output must survive the refused run untouched, got %q, %v", got, readErr)
	}
}

// TestRunToolStepCleansFreshOutputOnFailure checks that a step's first-ever run also treats its declared outputs as a unit, removing a partial output on failure.
func TestRunToolStepCleansFreshOutputOnFailure(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	g := &buildgraph.Graph{
		Roots: []string{outDir},
		Files: []buildgraph.FileRef{
			{ID: 0, Kind: "toolOutput", Path: "out1.txt", ProducedBy: 0},
			{ID: 1, Kind: "toolOutput", Path: "out2.txt", ProducedBy: 0},
		},
		ToolSteps: []buildgraph.ToolStep{
			{ID: 0, Framework: 0, Tool: "partial", Outputs: []int{0, 1}, Args: []string{"$out:0"}},
		},
	}
	rg := &ResolvedGraph{
		Graph: g,
		Frameworks: []FrameworkDescriptor{{Name: "fw", Tools: map[string]ToolProgram{
			"partial": {Path: "/bin/sh", Args: []string{"-c", `echo x > "$1"; exit 1`, "sh"}},
		}}},
	}
	filePath := func(id int) string { return filepath.Join(outDir, g.Files[id].Path) }
	var stderr bytes.Buffer
	if err := runToolStep(rg, 0, filePath, ExecOptions{}, outDir, nil, &stderr); err == nil {
		t.Fatal("a step that fails before writing all its outputs must be refused")
	}
	if _, err := os.Stat(filePath(0)); !os.IsNotExist(err) {
		t.Fatalf("out1.txt (fresh, no previous version) must not survive the failed run, stat err = %v", err)
	}
	if _, err := os.Stat(filePath(1)); !os.IsNotExist(err) {
		t.Fatalf("out2.txt must not exist, stat err = %v", err)
	}
}
