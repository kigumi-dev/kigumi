package vcs

import (
	"os"
	"strconv"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/toolchain/module"
)

// RemoteTagCommit resolves r's tag on the remote without cloning it.
func (g Git) RemoteTagCommit(r module.Require) (string, error) {
	url, subdir, ok := module.RepoOf(r)
	if !ok {
		return "", errors.NewErrf("dependency %s %s: cannot derive its repository; give `url` in the Require", r.Name, r.Version)
	}
	if err := checkRequire(r, url); err != nil {
		return "", err
	}
	tag := module.TagOf(subdir, r.Version)
	ref := "refs/tags/" + tag
	ls, err := g.command("ls-remote", "--tags", "--", url, ref, ref+"^{}")
	if err != nil {
		return "", err
	}
	out, err := ls.Output()
	if err != nil {
		return "", errors.WrapErrf(err, "git ls-remote %s", url)
	}
	var plain string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		commit, got, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		// ^{} is the annotated tag's dereferenced commit; prefer it.
		if got == ref+"^{}" {
			return commit, nil
		}
		if got == ref {
			plain = commit
		}
	}
	if plain == "" {
		return "", errors.NewErrf("%s has no tag %s", url, tag)
	}
	return plain, nil
}

// Refetch clones r fresh and swaps it in for dir, aside-then-remove so a crash between the two renames leaves the old entry recoverable.
func (g Git) Refetch(r module.Require, dir string) (string, error) {
	tmp := dir + ".refetch-tmp"
	os.RemoveAll(tmp)
	commit, err := g.Clone(r, tmp)
	if err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	aside := dir + ".stale-" + strconv.Itoa(os.Getpid())
	if err := os.Rename(dir, aside); err != nil {
		os.RemoveAll(tmp)
		return "", errors.WrapErr(err, "set aside the previous cache entry")
	}
	if err := os.Rename(tmp, dir); err != nil {
		os.Rename(aside, dir) // best-effort restore of the known-good tree
		return "", errors.WrapErr(err, "publish the updated cache entry")
	}
	os.RemoveAll(aside)
	return commit, nil
}
