package get

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

// target is what `kigumi get` operates on: a module root with mod.kg, or a script carrying manifest records.
type target struct {
	dir, script, lockPath string
	manifest              driver.ModFile
}

func openTarget(path string, lock bool) (*target, error) {
	if strings.HasSuffix(path, ".kg") {
		mf, _, err := driver.ReadScriptManifest(path)
		if err != nil {
			return nil, errors.WrapErr(err, "read script manifest")
		}
		t := &target{dir: filepath.Dir(path), script: path, manifest: mf}
		if lock || fileExists(path+".lock.kg") {
			t.lockPath = path + ".lock.kg"
		}
		return t, nil
	}
	mf, ok, err := driver.ReadModFile(path)
	if err != nil {
		return nil, errors.WrapErr(err, "read manifest")
	}
	if !ok {
		return nil, &exit.UsageError{Err: errors.NewErrf("%s has no %s; run `kigumi mod init` or name a script", path, driver.ManifestName)}
	}
	return &target{dir: path, manifest: mf, lockPath: filepath.Join(path, driver.LockName)}, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// add appends a Require for `path[@version]`, asking the repo for its newest tag when no version is given.
func (t *target) add(io session.IO, spec, url string) error {
	name, version, _ := strings.Cut(spec, "@")
	r := driver.Require{Name: name, Version: version, URL: url}
	if _, _, ok := driver.RepoOf(r); !ok {
		return &exit.UsageError{Err: errors.NewErrf("cannot derive where %s lives; give --url", name)}
	}
	if r.Version == "" {
		v, err := latestTag(r)
		if err != nil {
			return err
		}
		r.Version = v
	}
	var err error
	if t.script != "" {
		err = driver.AppendScriptRequire(t.script, r)
	} else {
		err = driver.AppendRequire(t.dir, r)
	}
	if err != nil {
		return &exit.UsageError{Err: err}
	}
	fmt.Fprintf(io.Msg, "added %s %s\n", r.Name, r.Version)
	t.manifest.Requires = append(t.manifest.Requires, r)
	return nil
}

func (t *target) fetch(io session.IO, opts options) error {
	lock := driver.Lock{Entries: map[string]driver.LockEntry{}}
	if t.lockPath != "" {
		var err error
		if lock, err = driver.ReadLockFile(t.lockPath); err != nil {
			return errors.WrapErr(err, "read lock")
		}
	}
	local, err := driver.ReadLocal(session.CompilerOptions(), t.dir)
	if err != nil {
		return errors.WrapErr(err, "read local overrides")
	}
	if session.LocalOff() {
		local = driver.Local{Replaces: map[string]string{}}
	}
	if opts.session.NoLocal {
		local = driver.Local{Replaces: map[string]string{}}
	}
	before := snapshotNative(lock)
	f := &fetcher{io: io, update: opts.update, lock: lock, local: local, next: driver.Lock{Entries: map[string]driver.LockEntry{}}, after: map[string]driver.ModFile{}}
	if err := f.module(t.manifest); err != nil {
		return err
	}
	printNativeChanges(io.Msg, diffNative(before, f.after), opts.diffJSON)
	if t.lockPath == "" {
		return nil
	}
	if err := driver.WriteLockFile(t.lockPath, f.next); err != nil {
		return errors.WrapErr(err, "write lock")
	}
	return nil
}
