package get

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/module/vcs"
)

// target is what `kigumi get` operates on: a module root with mod.kg, or a script carrying manifest records.
type target struct {
	dir, script, lockPath string
	manifest              module.ModFile
}

func openTarget(path string, lock bool) (*target, error) {
	if strings.HasSuffix(path, ".kg") {
		mf, _, err := module.ReadScriptManifest(path)
		if err != nil {
			return nil, errors.WrapErr(err, "read script manifest")
		}
		t := &target{dir: filepath.Dir(path), script: path, manifest: mf}
		if lock || fileExists(path+".lock.kg") {
			t.lockPath = path + ".lock.kg"
		}
		return t, nil
	}
	mf, ok, err := module.ReadModFile(path)
	if err != nil {
		return nil, errors.WrapErr(err, "read manifest")
	}
	if !ok {
		return nil, &exit.UsageError{Err: errors.NewErrf("%s has no %s; run `kigumi mod init` or name a script", path, module.ManifestName)}
	}
	return &target{dir: path, manifest: mf, lockPath: filepath.Join(path, module.LockName)}, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// add appends a Require for `path[@version]`, asking the repo for its newest tag when no version is given.
func (t *target) add(io session.IO, git vcs.Git, spec, url string) error {
	name, version, _ := strings.Cut(spec, "@")
	r := module.Require{Name: name, Version: version, URL: url}
	if _, _, ok := module.RepoOf(r); !ok {
		return &exit.UsageError{Err: errors.NewErrf("cannot derive where %s lives; give --url", name)}
	}
	if r.Version == "" {
		v, err := git.LatestTag(r)
		if err != nil {
			return err
		}
		r.Version = v
	}
	var err error
	if t.script != "" {
		err = module.AppendScriptRequire(t.script, r)
	} else {
		err = module.AppendRequire(t.dir, r)
	}
	if err != nil {
		return &exit.UsageError{Err: err}
	}
	fmt.Fprintf(io.Err, "added %s %s\n", r.Name, r.Version)
	t.manifest.Requires = append(t.manifest.Requires, r)
	return nil
}

func (t *target) fetch(io session.IO, git vcs.Git, opts options) error {
	lock := module.Lock{Entries: map[string]module.LockEntry{}}
	if t.lockPath != "" {
		var err error
		if lock, err = module.ReadLockFile(t.lockPath); err != nil {
			return errors.WrapErr(err, "read lock")
		}
	}
	local, err := opts.session.ReadLocal(t.dir)
	if err != nil {
		return errors.WrapErr(err, "read local overrides")
	}
	before := snapshotNative(lock)
	f := &fetcher{io: io, git: git, update: opts.update, lock: lock, local: local, next: module.Lock{Entries: map[string]module.LockEntry{}}, after: map[string]module.ModFile{}}
	if err := f.module(t.manifest); err != nil {
		return err
	}
	printNativeChanges(io, diffNative(before, f.after), opts.diffJSON)
	if t.lockPath == "" {
		return nil
	}
	if err := module.WriteLockFile(t.lockPath, f.next); err != nil {
		return errors.WrapErr(err, "write lock")
	}
	return nil
}
