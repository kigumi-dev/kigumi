package get

import (
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/module/vcs"
)

type fetcher struct {
	io     session.IO
	git    vcs.Git
	update bool
	// lock is what the lock file said before; next is what it will say.
	lock, next module.Lock
	local      module.Local
	// after collects each resolved dependency's native declarations for the consent diff against snapshotNative.
	after map[string]module.ModFile
}

// module fetches until minimal version selection closes; a mod.local.kg Replace applies to the root only and is never locked.
func (f *fetcher) module(mf module.ModFile) error {
	var sel *module.Selection
	for {
		var err error
		sel, err = module.SelectVersions(session.CompilerOptions(), mf, f.local)
		if err == nil {
			break
		}
		var missing *module.MissingError
		if !stderrors.As(err, &missing) {
			return err
		}
		dir, _, err := module.DepDir(session.CompilerOptions(), f.local, missing.Require, true)
		if err != nil {
			return err
		}
		if err := f.cached(missing.Require, dir, true); err != nil {
			return err
		}
	}
	for _, name := range sel.Names() {
		r := sel.Requires[name]
		dir, missing, err := module.DepDir(session.CompilerOptions(), f.local, r, true)
		if err != nil {
			return errors.WrapErr(err, "resolve dependency")
		}
		if _, replaced := f.local.Replaces[r.Name]; replaced {
			fmt.Fprintf(f.io.Msg, "%s %s: %s (replace)\n", r.Name, r.Version, dir)
		} else if err := f.cached(r, dir, missing); err != nil {
			return err
		}
		if depMf, ok, err := module.ReadModFile(dir); err != nil {
			return errors.WrapErr(err, "read dependency manifest")
		} else if ok {
			f.native(r.Name, depMf)
		}
	}
	return nil
}

// native lists what a dependency adds to the link line, so the root sees what it trusts, and records it for the diff.
func (f *fetcher) native(name string, mf module.ModFile) {
	if f.after != nil {
		f.after[name] = mf
	}
	for _, l := range mf.Links {
		if l.Library != "" {
			fmt.Fprintf(f.io.Msg, "  %s links -l%s\n", name, l.Library)
		}
		if l.Search != "" {
			fmt.Fprintf(f.io.Msg, "  %s adds search path %s\n", name, l.Search)
		}
	}
	for _, s := range mf.Sources {
		fmt.Fprintf(f.io.Msg, "  %s compiles %s\n", name, s.Path)
	}
}

// cached makes sure the cache holds r and that it agrees with the lock.
func (f *fetcher) cached(r module.Require, dir string, missing bool) error {
	if done, ok := f.next.Entries[r.Name]; ok && done.Version == r.Version {
		return nil
	}
	prev, locked := f.lock.Entries[r.Name]
	locked = locked && prev.Version == r.Version
	if want := module.RepoURLKey(r); locked && !f.update && prev.URL != "" && want != "" && prev.URL != want {
		return errors.NewErrf("%s %s is now fetched from %s but the lock recorded %s; run `kigumi get -u` to accept the new source", r.Name, r.Version, want, prev.URL)
	}
	if missing {
		rawCommit, err := f.git.Clone(r, dir)
		if err != nil {
			return err
		}
		commit := module.TagCommit(rawCommit)
		if locked && !f.update && prev.Commit != commit {
			os.RemoveAll(dir)
			return errors.NewErrf("%s %s now points at %s but the lock recorded %s; run `kigumi get -u` to accept the new commit", r.Name, r.Version, short(commit), short(prev.Commit))
		}
		fmt.Fprintf(f.io.Msg, "%s %s: fetched into %s\n", r.Name, r.Version, dir)
	} else if f.update && locked && !module.IsCommitHash(r.Version) {
		remote, err := f.git.RemoteTagCommit(r)
		if err != nil {
			return errors.WrapErrf(err, "%s %s: check the remote tag", r.Name, r.Version)
		}
		if module.TagCommit(remote) == module.NormalizeCommit(prev.Commit) {
			fmt.Fprintf(f.io.Msg, "%s %s: %s (up to date)\n", r.Name, r.Version, dir)
		} else if newCommit, err := f.git.Refetch(r, dir); err != nil {
			return err
		} else {
			fmt.Fprintf(f.io.Msg, "%s %s: moved from %s to %s; refetched into %s\n", r.Name, r.Version, short(prev.Commit), short(newCommit), dir)
			missing = true
		}
	} else {
		fmt.Fprintf(f.io.Msg, "%s %s: %s\n", r.Name, r.Version, dir)
	}
	hash, err := module.TreeHash(dir)
	if err != nil {
		return errors.WrapErr(err, "hash dependency")
	}
	if locked && prev.Hash != hash && !(missing && f.update) {
		return errors.NewErrf("%s %s in %s does not match the lock; remove that directory and run `kigumi get` again", r.Name, r.Version, dir)
	}
	commit, err := os.ReadFile(filepath.Join(dir, module.OriginFileName))
	if err != nil {
		return errors.WrapErrf(err, "%s %s: read the origin marker", r.Name, r.Version)
	}
	f.next.Entries[r.Name] = module.LockEntry{Version: r.Version, Commit: module.TagCommit(strings.TrimSpace(string(commit))), Hash: hash, URL: module.RepoURLKey(r)}
	return nil
}

func short(commit string) string {
	algo, hex, ok := strings.Cut(commit, ":")
	if !ok {
		hex, algo = algo, ""
	}
	if len(hex) > 12 {
		hex = hex[:12] + "…"
	}
	if algo == "" {
		return hex
	}
	return algo + ":" + hex
}
