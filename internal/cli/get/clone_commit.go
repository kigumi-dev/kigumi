package get

import (
	"os"

	"github.com/spf13/cobra"

	"kigumi/internal/errors"
)

// shallowFetch is a var, not a direct call, so tests can make it fail without a real server.
var shallowFetch = shallowFetchCommit

func cloneCommit(cmd *cobra.Command, url, hash, tmp string) (string, error) {
	if len(hash) != 40 && len(hash) != 64 {
		return cloneCommitFull(cmd, url, hash, tmp)
	}
	full, err := shallowFetch(cmd, url, hash, tmp)
	if err == nil {
		return full, nil
	}
	cmd.Printf("%s: cannot fetch commit %s directly (%s); cloning the full history instead\n", url, hash, err)
	if err := os.RemoveAll(tmp); err != nil {
		return "", errors.WrapErr(err, "clear partial fetch")
	}
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return "", errors.WrapErr(err, "clear partial fetch")
	}
	return cloneCommitFull(cmd, url, hash, tmp)
}

func shallowFetchCommit(cmd *cobra.Command, url, hash, tmp string) (string, error) {
	if _, err := git(cmd, "", "init", "--quiet", "--", tmp); err != nil {
		return "", err
	}
	if _, err := git(cmd, tmp, "remote", "add", "--", "origin", url); err != nil {
		return "", err
	}
	if _, err := git(cmd, tmp, "fetch", "--quiet", "--depth", "1", "--", "origin", hash); err != nil {
		return "", err
	}
	full, err := git(cmd, tmp, "rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", err
	}
	if _, err := git(cmd, tmp, "checkout", "--quiet", full); err != nil {
		return "", err
	}
	return full, nil
}

func cloneCommitFull(cmd *cobra.Command, url, hash, tmp string) (string, error) {
	if _, err := git(cmd, "", "clone", "--quiet", "--no-checkout", "--", url, tmp); err != nil {
		return "", errors.WrapErrf(err, "git clone %s", url)
	}
	full, err := git(cmd, tmp, "rev-parse", "--verify", "--quiet", hash+"^{commit}")
	if err != nil {
		return "", errors.NewErrf("%s has no commit %s (or it is ambiguous)", url, hash)
	}
	if _, err := git(cmd, tmp, "checkout", "--quiet", full); err != nil {
		return "", errors.WrapErrf(err, "git checkout %s", full)
	}
	return full, nil
}
