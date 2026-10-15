package driver

import (
	"encoding/json"

	"kigumi/internal/diag"
	"kigumi/internal/sem"
)

// PackageStatus is one package's outcome in the last recording: its key, the digest of its API artifact, whether the record already existed, and whether an artifact was written.
type PackageStatus struct {
	Path   string
	Key    string
	Digest string
	Hit    bool
	// Written marks an artifact body new to the store.
	Written bool
}

// PackageStatuses lists the last recording in dependency order; nil when the cache is off or nothing was recorded.
func (m *Module) PackageStatuses() []PackageStatus { return m.pkgStatus }

func packageHasErrors(res *sem.Result, path string) bool {
	pkg := res.PackageByPath(path)
	if pkg == 0 {
		return false
	}
	for _, f := range res.Packages[pkg].Files {
		for _, d := range res.Files[f].Diags {
			if d.Severity == diag.Error {
				return true
			}
		}
	}
	return false
}

// recordPackages writes missing artifacts dependencies first, so a dependant keys on the digests its imports settled on; an erroring package gets none.
func (m *Module) recordPackages(res *sem.Result) {
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
	digests := map[string]string{}
	for _, comp := range g.sccs {
		for _, p := range comp {
			key, err := m.packageKey(g, p, env, std, digests)
			if err != nil {
				return
			}
			st := PackageStatus{Path: p, Key: key}
			if rec, ok := c.lookup(key); ok {
				c.stats.ArtifactHits++
				st.Hit, st.Digest = true, rec.Digest
			} else if packageHasErrors(res, p) {
				st.Digest = "error"
			} else if api, ok := res.PackageAPIOf(p); !ok {
				st.Digest = "absent"
			} else if body, err := json.Marshal(api); err == nil {
				if digest, fresh, ok := c.putAPI(body); ok && c.putSealed("pkg", key, keyRecord{Path: p, Digest: digest}) {
					c.stats.RecordWrites++
					st.Digest, st.Written = digest, fresh
					if fresh {
						c.stats.ArtifactWrites++
					}
				}
			}
			digests[p] = st.Digest
			m.pkgStatus = append(m.pkgStatus, st)
		}
	}
}
