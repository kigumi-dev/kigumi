package vcs

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/toolchain/module"
)

// Clone fetches the version of r into dir through a temporary directory, so a failed fetch leaves no half-written
// module, and returns the commit it resolved to.
func (g Git) Clone(r module.Require, dir string) (string, error) {
	url, subdir, ok := module.RepoOf(r)
	if !ok {
		return "", errors.NewErrf("dependency %s %s: cannot derive its repository; give `url` in the Require", r.Name, r.Version)
	}
	if err := checkRequire(r, url); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", errors.WrapErr(err, "create module cache")
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), "fetch-")
	if err != nil {
		return "", errors.WrapErr(err, "create module cache")
	}
	defer os.RemoveAll(tmp)
	var commit string
	if module.IsCommitHash(r.Version) {
		commit, err = g.cloneCommit(url, r.Version, tmp)
	} else {
		commit, err = g.cloneTag(url, module.TagOf(subdir, r.Version), tmp)
	}
	if err != nil {
		return "", err
	}
	src, err := confinedSubdir(tmp, subdir, r)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(src, module.ManifestName)); err != nil {
		return "", errors.NewErrf("%s at %s has no %s in %q", r.Name, r.Version, module.ManifestName, subdir)
	}
	os.RemoveAll(filepath.Join(tmp, ".git"))
	if err := dropNestedModules(src); err != nil {
		return "", err
	}
	if err := module.CheckCacheTree(src); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(src, module.OriginFileName), []byte(commit+"\n"), 0o644); err != nil {
		return "", errors.WrapErr(err, "record origin")
	}
	if err := os.Rename(src, dir); err != nil {
		// Another process may have published the same entry meanwhile.
		if module.CacheReady(dir) {
			return commit, nil
		}
		return "", errors.WrapErr(err, "store fetched module")
	}
	return commit, nil
}

func (g Git) cloneTag(url, tag, tmp string) (string, error) {
	if _, err := g.run("", "clone", "--quiet", "--depth", "1", "--branch", tag, "--", url, tmp); err != nil {
		return "", errors.WrapErrf(err, "git clone %s at %s", url, tag)
	}
	head, err := g.run(tmp, "rev-parse", "HEAD")
	if err != nil {
		return "", errors.WrapErr(err, "git rev-parse")
	}
	// --branch also matches a branch of that name; insist on the tag.
	if tagged, err := g.run(tmp, "rev-parse", "--verify", "--quiet", "refs/tags/"+tag+"^{commit}"); err != nil || tagged != head {
		return "", errors.NewErrf("%s has no tag %s (a branch of that name is not a version)", url, tag)
	}
	return head, nil
}

// dropNestedModules removes module roots below src: they are separate modules, versioned and fetched on their own.
func dropNestedModules(src string) error {
	var nested []string
	filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == src {
			return nil
		}
		if module.IsModuleRoot(path) {
			nested = append(nested, path)
			return fs.SkipDir
		}
		return nil
	})
	for _, n := range nested {
		if err := os.RemoveAll(n); err != nil {
			return errors.WrapErr(err, "drop nested module")
		}
	}
	return nil
}

// LatestTag picks the highest release tag (prereleases excluded) of the module's subdirectory in its repo.
func (g Git) LatestTag(r module.Require) (string, error) {
	url, subdir, _ := module.RepoOf(r)
	if err := checkRequire(r, url); err != nil {
		return "", err
	}
	ls, err := g.command("ls-remote", "--tags", "--refs", "--", url)
	if err != nil {
		return "", err
	}
	out, err := ls.Output()
	if err != nil {
		return "", errors.WrapErrf(err, "git ls-remote %s", url)
	}
	prefix := "refs/tags/" + module.TagOf(subdir, "")
	var versions []string
	for _, line := range bytes.Split(out, []byte("\n")) {
		_, ref, ok := strings.Cut(string(line), "\t")
		if v, has := strings.CutPrefix(ref, prefix); ok && has && strings.HasPrefix(v, "v") && !strings.Contains(v, "-") && !strings.Contains(v, "/") {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return "", errors.NewErrf("%s has no release tag for %s", url, r.Name)
	}
	sort.Slice(versions, func(i, j int) bool { return module.CompareVersions(versions[i], versions[j]) < 0 })
	return versions[len(versions)-1], nil
}
