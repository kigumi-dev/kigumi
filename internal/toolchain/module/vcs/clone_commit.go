package vcs

import (
	"fmt"
	"os"

	"kigumi/internal/base/errors"
)

// shallowFetch is a var, not a direct call, so tests can make it fail without a real server.
var shallowFetch = shallowFetchCommit

func (g Git) cloneCommit(url, hash, tmp string) (string, error) {
	if len(hash) != 40 && len(hash) != 64 {
		return g.cloneCommitFull(url, hash, tmp)
	}
	full, err := shallowFetch(g, url, hash, tmp)
	if err == nil {
		return full, nil
	}
	if g.Log != nil {
		fmt.Fprintf(g.Log, "%s: cannot fetch commit %s directly (%s); cloning the full history instead\n", url, hash, err)
	}
	if err := os.RemoveAll(tmp); err != nil {
		return "", errors.WrapErr(err, "clear partial fetch")
	}
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return "", errors.WrapErr(err, "clear partial fetch")
	}
	return g.cloneCommitFull(url, hash, tmp)
}

func shallowFetchCommit(g Git, url, hash, tmp string) (string, error) {
	if _, err := g.run("", "init", "--quiet", "--", tmp); err != nil {
		return "", err
	}
	if _, err := g.run(tmp, "remote", "add", "--", "origin", url); err != nil {
		return "", err
	}
	if _, err := g.run(tmp, "fetch", "--quiet", "--depth", "1", "--", "origin", hash); err != nil {
		return "", err
	}
	full, err := g.run(tmp, "rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", err
	}
	if _, err := g.run(tmp, "checkout", "--quiet", full); err != nil {
		return "", err
	}
	return full, nil
}

func (g Git) cloneCommitFull(url, hash, tmp string) (string, error) {
	if _, err := g.run("", "clone", "--quiet", "--no-checkout", "--", url, tmp); err != nil {
		return "", errors.WrapErrf(err, "git clone %s", url)
	}
	full, err := g.run(tmp, "rev-parse", "--verify", "--quiet", hash+"^{commit}")
	if err != nil {
		return "", errors.NewErrf("%s has no commit %s (or it is ambiguous)", url, hash)
	}
	if _, err := g.run(tmp, "checkout", "--quiet", full); err != nil {
		return "", errors.WrapErrf(err, "git checkout %s", full)
	}
	return full, nil
}
