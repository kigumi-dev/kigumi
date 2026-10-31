// e2e_*.go implements the end-to-end benchmark runner.
package testtool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"kigumi/internal/toolchain/version"
)

// E2EOptions configures RunE2E.
type E2EOptions struct {
	// RepoRoot holds go.mod and the kigumi main package.
	RepoRoot string
	// Module is the Kigumi module or example to check, build and run.
	Module  string
	StdRoot string
	// WorkDir holds the built compiler and the build artifact; the caller
	// owns its lifetime.
	WorkDir string
	// Lookup finds the go tool; nil finds nothing.
	Lookup Lookup
}

// RunE2E builds the compiler once, measures each phase, and writes one
// E2EEvent per phase as JSON Lines to w, stopping at the first error.
func RunE2E(opts E2EOptions, w io.Writer) error {
	enc := json.NewEncoder(w)
	toolchainCommit := version.Current().Commit
	emit := func(phase string, dur time.Duration, extra func(*E2EEvent)) {
		e := E2EEvent{
			FormatVersion: E2EFormatVersion, Kind: "e2e-bench", Phase: phase, Module: opts.Module,
			Timestamp: time.Now().UTC(), DurationMS: float64(dur.Microseconds()) / 1000, OK: true,
			OS: runtime.GOOS, Arch: runtime.GOARCH, NumCPU: runtime.NumCPU(), ToolchainBuild: toolchainCommit,
		}
		if extra != nil {
			extra(&e)
		}
		enc.Encode(e)
	}
	fail := func(phase string, dur time.Duration, err error) error {
		emit(phase, dur, func(e *E2EEvent) { e.OK = false; e.Error = err.Error() })
		return fmt.Errorf("%s: %w", phase, err)
	}

	exe := filepath.Join(opts.WorkDir, "kigumi-e2e")
	start := time.Now()
	if opts.Lookup == nil {
		return fail("compiler-build", time.Since(start), fmt.Errorf("go not found: no tool search path given"))
	}
	goTool, err := opts.Lookup("go")
	if err != nil {
		return fail("compiler-build", time.Since(start), err)
	}
	build := exec.Command(goTool, "build", "-o", exe, ".")
	build.Dir = opts.RepoRoot
	if out, err := build.CombinedOutput(); err != nil {
		return fail("compiler-build", time.Since(start), fmt.Errorf("%w\n%s", err, out))
	}
	emit("compiler-build", time.Since(start), nil)

	for _, phase := range []string{"check-cold", "check-warm"} {
		dur, rss, err := runPhase(exe, opts.RepoRoot, "--std", opts.StdRoot, "check", opts.Module)
		if err != nil {
			return fail(phase, dur, err)
		}
		emit(phase, dur, func(e *E2EEvent) { e.MaxRSSKiB = rss })
	}

	artifact := filepath.Join(opts.WorkDir, "e2e-artifact")
	dur, rss, err := runPhase(exe, opts.RepoRoot, "--std", opts.StdRoot, "build", "-o", artifact, opts.Module)
	if err != nil {
		return fail("build-link", dur, err)
	}
	var size int64
	if st, statErr := os.Stat(artifact); statErr == nil {
		size = st.Size()
	}
	emit("build-link", dur, func(e *E2EEvent) { e.MaxRSSKiB = rss; e.ArtifactBytes = size })

	dur, rss, err = runPhase(artifact, opts.RepoRoot)
	if err != nil {
		return fail("run", dur, err)
	}
	emit("run", dur, func(e *E2EEvent) { e.MaxRSSKiB = rss })
	return nil
}

// runPhase runs exe, timing it and reading its peak RSS afterward.
func runPhase(exe, dir string, args ...string) (time.Duration, int64, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start)
	if err != nil {
		return dur, 0, fmt.Errorf("%w\n%s", err, stderr.String())
	}
	return dur, maxRSSKiB(cmd), nil
}
