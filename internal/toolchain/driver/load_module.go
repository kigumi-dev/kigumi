package driver

import (
	"os"
	"path/filepath"
	"time"

	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/policy"
)

// loadModule indexes every package under root and the std stubs, then parses the packages the root set reaches. Parse diagnostics stay on each Tree; loading itself only fails on I/O.
func loadModule(root string, opts LoadOptions) (*Module, error) {
	m := &Module{Root: root, Entry: opts.Entry, ExplicitEntry: opts.Entry != "", overlay: opts.Overlay, Packages: map[string]*Package{}, local: module.Local{Replaces: map[string]string{}}, target: opts.Target, deny: opts.Deny, ceiling: opts.Ceiling, artifactName: opts.ArtifactName, explicitZig: opts.ZigPath, zigHome: opts.ZigHome, zigIndexURL: opts.ZigIndexURL, zigPublicKey: opts.ZigPublicKey, offline: opts.Offline, zigNotice: opts.ZigNotice, verifyPasses: opts.VerifyPasses, rootDirs: opts.RootDirs, rootImports: opts.RootImports, allPackages: opts.AllPackages, allOwn: opts.AllOwnPackages, deferred: opts.DeferParse, unitCheck: opts.UnitCheck, lazyStd: opts.LazyStd, test: opts.Test, keyEntry: opts.Entry, keyExplicit: opts.Entry != "", fileOrder: opts.FileOrder, compiler: opts.Compiler, headers: headerLibc{opts: opts}}
	m.index = newPkgIndex(opts.PackageCache, root, opts.Compiler, time.Now)
	m.pkgCache = newPackageCache(opts.PackageCache)
	if m.target.OS == "" {
		m.target = m.compiler.HostOrAmbient()
	}
	if m.target.Unverified() {
		m.Violations = append(m.Violations, policy.UnverifiedTargetViolation(m.target.Triple))
	}
	mf, hasMod, err := module.ReadModFile(root)
	if err != nil {
		return nil, err
	}
	m.Name, m.Manifest = mf.Name, mf
	skip := m.skipper(root, opts)
	if err := m.indexTree(root, m.Name, false, opts.Test, skip); err != nil {
		return nil, err
	}
	// A script's manifest records live in a parsed file; a module's manifest is mod.kg.
	if !m.deferred || !hasMod {
		if err := m.parseReachable(); err != nil {
			return nil, err
		}
		if err := m.scriptManifest(hasMod); err != nil {
			return nil, err
		}
	}
	if err := m.addNative(root, m.Manifest); err != nil {
		return nil, err
	}
	m.addExtraNative(opts)
	if err := m.addHeaders(root, m.Manifest); err != nil {
		return nil, err
	}
	if !opts.NoLocal {
		if m.local, err = module.ReadLocal(opts.Compiler, root); err != nil {
			return nil, err
		}
		if err := m.checkLocalReplace(opts.Policy); err != nil {
			return nil, err
		}
	}
	if !opts.NoDeps {
		lock, err := module.ReadLock(root)
		if err != nil {
			return nil, err
		}
		enforceLock := true
		if m.ScriptFile != "" {
			scriptLock := filepath.Join(root, filepath.FromSlash(m.ScriptFile)) + ".lock.kg"
			if _, statErr := os.Stat(scriptLock); statErr != nil {
				if len(m.Manifest.Requires) > 0 {
					v := policy.UnlockedScriptViolation(m.ScriptFile, filepath.ToSlash(scriptLock))
					if opts.Policy.Dependency == policy.DependencyLocked {
						return nil, policy.Error(v)
					}
					m.Violations = append(m.Violations, v)
				}
				enforceLock = false
			} else if lock, err = module.ReadLockFile(scriptLock); err != nil {
				return nil, err
			}
		}
		if err := m.loadDeps(opts, lock, enforceLock); err != nil {
			return nil, err
		}
	}
	if opts.StdRoot != "" {
		if err := m.indexTree(opts.StdRoot, "std", true, false, nil); err != nil {
			return nil, err
		}
	}
	if m.deferred {
		if m.trySettle(); m.settled == nil {
			m.ensureParsed()
		}
	} else if err := m.parseReachable(); err != nil {
		return nil, err
	} else {
		m.setOrder()
	}
	m.saveMemo()
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	if m.fileOrder != nil {
		m.fileOrder(m.Order)
	}
	return m, nil
}
