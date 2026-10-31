package source

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"kigumi/internal/toolchain/zig/store"
)

// ErrVersionMismatch marks a build that completed but whose binary does not
// report exactly the requested version; the install is always rejected.
var ErrVersionMismatch = errors.New("built zig does not report the requested version")

// VersionMismatch is ErrVersionMismatch carrying the reported and requested
// versions separately, for callers that report them in their own format.
type VersionMismatch struct {
	Got, Want string
}

func (e *VersionMismatch) Error() string {
	return fmt.Sprintf("%s: got %q, want %q", ErrVersionMismatch, e.Got, e.Want)
}

func (e *VersionMismatch) Unwrap() error { return ErrVersionMismatch }

// Build runs req's recipe end to end and installs into storeRoot; any step
// failing, including version verification, discards staging, so nothing partial is ever committed.
func Build(req Request, storeRoot string, stderr func(string)) (store.Manifest, error) {
	if req.Jobs <= 0 {
		req.Jobs = runtime.NumCPU()
	}
	st, err := store.Stage(storeRoot, req.Want)
	if err != nil {
		return store.Manifest{}, err
	}
	defer st.Discard()
	req.StageDir = st.Dir

	steps, err := Plan(req)
	if err != nil {
		return store.Manifest{}, err
	}
	for _, step := range steps {
		if stderr != nil {
			stderr("kigumi: " + strings.Join(step.Argv, " "))
		}
		prog, err := lookup(req.Lookup, step.Argv[0])
		if err != nil {
			return store.Manifest{}, err
		}
		cmd := exec.Command(prog, step.Argv[1:]...)
		cmd.Dir = step.Dir
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			return store.Manifest{}, fmt.Errorf("%s: %w: %s", strings.Join(step.Argv, " "), err, out.String())
		}
	}

	got, err := exec.Command(store.BinaryPath(st.Dir), "version").Output()
	if err != nil {
		return store.Manifest{}, fmt.Errorf("built zig did not run `version`: %w", err)
	}
	version := strings.TrimSpace(string(got))
	if version != req.Want {
		return store.Manifest{}, &VersionMismatch{Got: version, Want: req.Want}
	}

	commit, dirty := gitState(req.Lookup, req.Checkout)
	m := store.Manifest{
		Version:     req.Want,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		InstalledAt: time.Now().UTC(),
		Obtained:    "source",
		Commit:      commit,
		Dirty:       dirty,
	}
	if err := store.WriteManifest(st.Dir, m); err != nil {
		return store.Manifest{}, err
	}
	if err := st.Commit(); err != nil {
		return store.Manifest{}, err
	}
	return m, nil
}

func gitState(look func(name string) (string, error), checkout string) (commit string, dirty bool) {
	git, err := lookup(look, "git")
	if err != nil {
		return "", false
	}
	out, err := exec.Command(git, "-C", checkout, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", false
	}
	commit = strings.TrimSpace(string(out))
	status, err := exec.Command(git, "-C", checkout, "status", "--porcelain").Output()
	return commit, err == nil && len(bytes.TrimSpace(status)) > 0
}

func lookup(look func(name string) (string, error), name string) (string, error) {
	if look == nil {
		return "", fmt.Errorf("%s not found: no tool search path given", name)
	}
	return look(name)
}
