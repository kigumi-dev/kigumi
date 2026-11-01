package get

import (
	"fmt"
	"os"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
)

// shallowFetch is a var, not a direct call, so tests can make it fail without a real server.
var shallowFetch = shallowFetchCommit

func cloneCommit(io session.IO, url, hash, tmp string) (string, error) {
	if len(hash) != 40 && len(hash) != 64 {
		return cloneCommitFull(io, url, hash, tmp)
	}
	full, err := shallowFetch(io, url, hash, tmp)
	if err == nil {
		return full, nil
	}
	fmt.Fprintf(io.Msg, "%s: cannot fetch commit %s directly (%s); cloning the full history instead\n", url, hash, err)
	if err := os.RemoveAll(tmp); err != nil {
		return "", errors.WrapErr(err, "clear partial fetch")
	}
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return "", errors.WrapErr(err, "clear partial fetch")
	}
	return cloneCommitFull(io, url, hash, tmp)
}

func shallowFetchCommit(io session.IO, url, hash, tmp string) (string, error) {
	if _, err := git(io, "", "init", "--quiet", "--", tmp); err != nil {
		return "", err
	}
	if _, err := git(io, tmp, "remote", "add", "--", "origin", url); err != nil {
		return "", err
	}
	if _, err := git(io, tmp, "fetch", "--quiet", "--depth", "1", "--", "origin", hash); err != nil {
		return "", err
	}
	full, err := git(io, tmp, "rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", err
	}
	if _, err := git(io, tmp, "checkout", "--quiet", full); err != nil {
		return "", err
	}
	return full, nil
}

func cloneCommitFull(io session.IO, url, hash, tmp string) (string, error) {
	if _, err := git(io, "", "clone", "--quiet", "--no-checkout", "--", url, tmp); err != nil {
		return "", errors.WrapErrf(err, "git clone %s", url)
	}
	full, err := git(io, tmp, "rev-parse", "--verify", "--quiet", hash+"^{commit}")
	if err != nil {
		return "", errors.NewErrf("%s has no commit %s (or it is ambiguous)", url, hash)
	}
	if _, err := git(io, tmp, "checkout", "--quiet", full); err != nil {
		return "", errors.WrapErrf(err, "git checkout %s", full)
	}
	return full, nil
}
