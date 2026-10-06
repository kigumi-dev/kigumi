// Package source builds zig from a git checkout for a pin with no prebuilt
// tarball, via the CMake+Ninja recipe at https://codeberg.org/ziglang/zig (needs a matching LLVM toolchain).
package source

import (
	"fmt"
	"strings"
)

// Request describes one source build.
type Request struct {
	// URL is a git remote to clone; "" builds Checkout in place instead.
	URL string
	// Ref is the branch, tag or commit to build; "" uses the checkout's
	// current HEAD (only meaningful with URL == "").
	Ref string
	// Checkout is the working directory: where URL is cloned to, or an
	// existing local checkout when URL == "".
	Checkout string
	// Want is the exact `zig version` output the built binary must report;
	// anything else rejects the install.
	Want string
	// StageDir is the install prefix the recipe's build step populates;
	// the caller stages it under the store and commits it after Verify.
	StageDir string
	// Jobs is the parallel build job count; the caller resolves 0 to
	// runtime.NumCPU() before calling Plan, so assembly stays deterministic.
	Jobs int
}

// Step is one command of the assembled recipe, run with Dir as its working
// directory ("" meaning the caller's own).
type Step struct {
	Dir  string
	Argv []string
}

// Plan assembles the recipe's steps without running anything, so the
// assembly itself is testable without a C++ toolchain or network access.
func Plan(req Request) ([]Step, error) {
	if req.Checkout == "" {
		return nil, fmt.Errorf("source build: Checkout is required")
	}
	if req.StageDir == "" {
		return nil, fmt.Errorf("source build: StageDir is required")
	}
	if strings.HasPrefix(req.URL, "-") || strings.HasPrefix(req.Ref, "-") {
		return nil, fmt.Errorf("source build: URL and Ref must not start with \"-\" (git would parse it as an option)")
	}
	var steps []Step
	if req.URL != "" {
		steps = append(steps, Step{Argv: []string{"git", "clone", "--quiet", "--", req.URL, req.Checkout}})
		if req.Ref != "" {
			steps = append(steps, Step{Dir: req.Checkout, Argv: []string{"git", "checkout", "--quiet", req.Ref}})
		}
	} else if req.Ref != "" {
		steps = append(steps, Step{Dir: req.Checkout, Argv: []string{"git", "checkout", "--quiet", req.Ref}})
	}
	build := req.Checkout + "/build"
	cmakeArgs := []string{"cmake", "-S", req.Checkout, "-B", build, "-DCMAKE_INSTALL_PREFIX=" + req.StageDir, "-DCMAKE_BUILD_TYPE=Release"}
	steps = append(steps, Step{Argv: cmakeArgs})
	jobs := req.Jobs
	if jobs <= 0 {
		jobs = 1
	}
	buildArgs := []string{"cmake", "--build", build, "--target", "install", "-j", fmt.Sprint(jobs)}
	steps = append(steps, Step{Argv: buildArgs})
	return steps, nil
}
