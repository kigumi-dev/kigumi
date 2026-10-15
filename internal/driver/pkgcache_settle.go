package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"kigumi/internal/diag"
)

// settledCheck is a clean check result kept under the module key: the diagnostics `kigumi check` prints, with nothing a later phase needs.
type settledCheck struct {
	ParseDiags string `json:"parseDiags"`
	Render     string `json:"render"`
}

// settledKey addresses a check outcome by its roots and every reached package record, so a failed or unrun package has no outcome to replay.
func (m *Module) settledKey() (string, bool) {
	g, err := m.keyGraph()
	if err != nil {
		return "", false
	}
	std, err := m.stdDigest()
	if err != nil {
		return "", false
	}
	env := m.cacheEnv()
	h := sha256.New()
	fmt.Fprintf(h, "env=%s\nstd=%s\nentry=%s,%v\nall=%v,%v\ncolor=%v\ndirs=%q\nimports=%q\n", env, std, m.keyEntry, m.keyExplicit, m.allPackages, m.allOwn, diag.Colored(), m.rootDirs, m.rootImports)
	digests := map[string]string{}
	for _, comp := range g.sccs {
		for _, p := range comp {
			key, err := m.packageKey(g, p, env, std, digests)
			if err != nil {
				return "", false
			}
			rec, ok := m.pkgCache.lookup(key)
			if !ok {
				return "", false
			}
			digests[p] = rec.Digest
			fmt.Fprintf(h, "pkg=%s:%s\n", p, key)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// trySettle replays a clean check when every reached package and the module itself are unchanged; it parses nothing.
func (m *Module) trySettle() {
	if m.pkgCache == nil {
		return
	}
	key, ok := m.settledKey()
	if !ok {
		return
	}
	var rec settledCheck
	if m.pkgCache.getSealed("check", key, &rec) {
		m.settled = &rec
		m.pkgCache.stats.SettledHits++
	}
}

// storeSettled keeps a clean check of this module for the next unchanged load. A module with comptime blocks is left out: they run code the key does not describe.
func (m *Module) storeSettled(a *Analysis, render string) {
	if m.pkgCache == nil || m.HasParseErrors() || a.Prog == nil || len(a.Prog.Comptime) > 0 {
		return
	}
	m.putSettled(render)
}

// putSettled stores the render under the module key, when every reached package has a record.
func (m *Module) putSettled(render string) {
	if m.pkgCache == nil {
		return
	}
	key, ok := m.settledKey()
	if !ok {
		return
	}
	if m.pkgCache.putSealed("check", key, settledCheck{ParseDiags: m.Diagnostics(), Render: render}) {
		m.pkgCache.stats.SettledWrites++
	}
}
