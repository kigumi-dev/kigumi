// Package vcs fetches module versions from their git repositories into the module cache.
package vcs

import (
	"io"
	"os/exec"
	"slices"
	"strings"

	"kigumi/internal/base/errors"
)

// Git runs git for module fetches; the caller resolves the executable and the environment, so nothing here reads process state.
type Git struct {
	Look func(name string) (string, error)
	// Env is git's whole environment; it must be set, since exec would read a nil Env as the process environment.
	Env    []string
	Stderr io.Writer
	// Log receives progress notes, such as falling back to a full clone; nil drops them.
	Log io.Writer
}

const allowedProtocols = "https:ssh:git:file"

// Protocols is the GIT_ALLOW_PROTOCOL value for a user setting: the default list, or the user's list narrowed to it.
func Protocols(user string, set bool) string {
	if !set {
		return allowedProtocols
	}
	var keep []string
	for _, p := range strings.Split(user, ":") {
		if slices.Contains(strings.Split(allowedProtocols, ":"), p) {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, ":")
}

func (g Git) command(args ...string) (*exec.Cmd, error) {
	if g.Look == nil || g.Env == nil {
		return nil, errors.NewErrf("vcs.Git needs Look and Env set")
	}
	path, err := g.Look("git")
	if err != nil {
		return nil, errors.WrapErr(err, "find git")
	}
	c := exec.Command(path, args...)
	c.Env = g.Env
	return c, nil
}

func (g Git) run(dir string, args ...string) (string, error) {
	c, err := g.command(args...)
	if err != nil {
		return "", err
	}
	c.Dir = dir
	c.Stderr = g.Stderr
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}
