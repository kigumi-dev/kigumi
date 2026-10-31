package driver

import (
	"encoding/json"

	"kigumi/internal/compiler/sem"
)

// PackageStatus is one package's outcome in the last recording: its key, the digest of its API artifact, whether the record already existed, and whether an artifact was written.
type PackageStatus struct {
	Path   string
	Key    string
	Digest string
	Hit    bool
	// Written marks an artifact body new to the store.
	Written bool
	// Replayed marks a package CheckUnits did not check because its record said it was clean.
	Replayed bool
	// Clean marks a package whose record says it checked with no diagnostic at all.
	Clean bool
}

// PackageStatuses lists the last recording in dependency order; nil when the cache is off or nothing was recorded.
func (m *Module) PackageStatuses() []PackageStatus { return m.pkgStatus }

func packageHasErrors(res *sem.Result, path string) bool {
	_, errs := res.PackageDiagCounts(path)
	return errs > 0
}

// unitOutcome is the run that checked a package; mir says the MIR phase ran, so its diagnostics are in res.
type unitOutcome struct {
	res      *sem.Result
	mir      bool
	comptime map[string]bool
}

// clean says the package produced nothing in a run that got through every phase, and holds no comptime block (the key does not describe what one reads).
func (m *Module) clean(o unitOutcome, path string) bool {
	if !o.mir || o.res.HasErrors() || o.comptime[path] {
		return false
	}
	if total, _ := o.res.PackageDiagCounts(path); total != 0 {
		return false
	}
	if p := m.Packages[path]; p != nil {
		for _, t := range p.Files {
			if len(t.Diags) != 0 {
				return false
			}
		}
	}
	return true
}

func digestUsable(d string) bool { return d != "" && d != "error" && d != "absent" }

// recordPackage stores the artifact and key record of one checked package, and corrects the clean mark of an existing record.
func (m *Module) recordPackage(o unitOutcome, path, key string, depsOK bool) PackageStatus {
	c := m.pkgCache
	st := PackageStatus{Path: path, Key: key}
	clean := depsOK && m.clean(o, path)
	st.Clean = clean
	if rec, ok := c.lookup(key); ok {
		c.stats.ArtifactHits++
		st.Hit, st.Digest = true, rec.Digest
		if clean != rec.Clean && c.putSealed("pkg", key, keyRecord{Path: path, Digest: rec.Digest, Clean: clean}) {
			c.stats.RecordWrites++
		}
		return st
	}
	if packageHasErrors(o.res, path) {
		st.Digest = "error"
		return st
	}
	api, ok := o.res.PackageAPIOf(path)
	if !ok {
		st.Digest = "absent"
		return st
	}
	body, err := json.Marshal(api)
	if err != nil {
		return st
	}
	if digest, fresh, ok := c.putAPI(body); ok && c.putSealed("pkg", key, keyRecord{Path: path, Digest: digest, Clean: clean}) {
		c.stats.RecordWrites++
		st.Digest, st.Written = digest, fresh
		if fresh {
			c.stats.ArtifactWrites++
		}
	}
	return st
}

// recordPackages writes missing artifacts dependencies first, so a dependant keys on the digests its imports settled on; an erroring package gets none. mir says the MIR phase ran over res.
func (m *Module) recordPackages(res *sem.Result, mir bool) {
	c := m.pkgCache
	m.pkgStatus = nil
	defer m.saveMemo()
	if c == nil || m.HasParseErrors() {
		return
	}
	g, err := m.keyGraph()
	if err != nil {
		return
	}
	std, err := m.stdDigest()
	if err != nil {
		return
	}
	env := m.cacheEnv()
	o := unitOutcome{res: res, mir: mir, comptime: res.ComptimePackages()}
	digests := map[string]string{}
	for _, comp := range g.sccs {
		for _, p := range comp {
			key, err := m.packageKey(g, p, env, std, digests)
			if err != nil {
				return
			}
			st := m.recordPackage(o, p, key, g.depsUsable(p, digests))
			digests[p] = st.Digest
			m.pkgStatus = append(m.pkgStatus, st)
		}
	}
}

func (g *pkgGraph) depsUsable(path string, digests map[string]string) bool {
	for _, dep := range g.closure(path) {
		if !digestUsable(digests[dep]) {
			return false
		}
	}
	return true
}
