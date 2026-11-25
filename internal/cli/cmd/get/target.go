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
		if lock || fileExists(path+".lock.kg") || manifestHasRange(mf) {
			t.lockPath = path + ".lock.kg"
		}
		return t, nil
	}
	mf, ok, err := module.ReadModFile(path)
	if err != nil {
		return nil, errors.WrapErr(err, "read manifest")
	}
	if !ok {
		return nil, &exit.ValueError{Err: errors.NewErrf("%s has no %s; run `kigumi mod init` or name a script", path, module.ManifestName)}
	}
	return &target{dir: path, manifest: mf, lockPath: filepath.Join(path, module.LockName)}, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func manifestHasRange(mf module.ModFile) bool {
	for _, r := range mf.Requires {
		if r.IsRange() {
			return true
		}
	}
	return false
}

// add appends a Require for `path[@version]`, asking the repo for its newest tag when no version is given.
func (t *target) add(io session.IO, git vcs.Git, spec, url, alias string) error {
	name, version, _ := strings.Cut(spec, "@")
	var rng *module.VersionRange
	if version != "" {
		var err error
		if rng, err = module.ParseVersionRequest(version); err != nil {
			return &exit.ValueError{Err: err}
		}
	}
	r := module.Require{Name: name, Version: version, URL: url, Alias: alias, Range: rng}
	if _, _, ok := module.RepoOf(r); !ok {
		return &exit.ValueError{Err: errors.NewErrf("cannot derive where %s lives; give --url", name)}
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
		return &exit.ValueError{Err: err}
	}
	fmt.Fprintf(io.Err, "added %s %s\n", module.DescribeRequire(r), r.Version)
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
	resolve := func(r module.Require) (module.Require, error) {
		if !r.IsRange() {
			return r, nil
		}
		if !opts.update {
			if e, ok := lock.Entries[r.Key()]; ok && (e.Name == "" || e.Name == r.Name) && e.Alias == r.Alias && r.Range.AcceptsString(e.Version) {
				return r.WithVersion(e.Version), nil
			}
		}
		v, err := git.LatestTagMatching(r, r.Range)
		if err != nil {
			return module.Require{}, err
		}
		return r.WithVersion(v), nil
	}
	before := snapshotNative(lock)
	f := &fetcher{io: io, git: git, update: opts.update, lock: lock, local: local, resolve: resolve, next: module.Lock{Entries: map[string]module.LockEntry{}}, after: map[string]module.ModFile{}}
	if err := f.module(t.manifest); err != nil {
		return err
	}
	printNativeChanges(io, diffNative(before, f.after), opts.diffJSON)
	if t.lockPath == "" {
		return nil
	}
	inputs, err := t.lockInputs(local)
	if err != nil {
		return err
	}
	state, err := module.SelectedLockState(inputs, f.next.Entries)
	if err != nil {
		return errors.WrapErr(err, "build lock")
	}
	if err := module.WriteLockStateFile(t.lockPath, state); err != nil {
		return errors.WrapErr(err, "write lock")
	}
	return nil
}

// lockInputs digests the bytes that made this fetch's selection: the manifest
// as written (the module's mod.kg or the script itself) and, when local
// overrides were read, their declarations. The digest is computed after any
// `get --add` edit, so the header never records stale inputs.
func (t *target) lockInputs(local module.Local) (string, error) {
	manifest := t.script
	if manifest == "" {
		manifest = filepath.Join(t.dir, module.ManifestName)
	}
	src, err := os.ReadFile(manifest)
	if err != nil {
		return "", errors.WrapErr(err, "read manifest bytes")
	}
	var localSrc []byte
	if local.File != "" {
		if localSrc, err = os.ReadFile(local.File); err != nil {
			return "", errors.WrapErr(err, "read local declarations")
		}
	}
	return module.LockInputsDigest(module.LockResolverOrderedV1, [][]byte{src}, localSrc), nil
}
