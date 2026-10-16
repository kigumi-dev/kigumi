package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"maps"
	"os"
	"slices"
	"strings"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/version"
)

// keyVersion moves when the key or artifact encoding changes, so older entries stop matching.
const keyVersion = 2

// pkgGraph is the import graph of the packages a key covers: own and dependency packages reachable from the roots, with same-module cycles collapsed into one component each.
type pkgGraph struct {
	nodes      []string
	deps       map[string][]string
	unresolved map[string][]string
	sccs       [][]string
	sccOf      map[string]int
}

// keyGraph walks the roots through scanned imports, never the parse, so an unparsed and a parsed load key the same way.
func (m *Module) keyGraph() (*pkgGraph, error) {
	reached, err := m.reach(m.rootPackages(), false, true)
	if err != nil {
		return nil, err
	}
	g := &pkgGraph{deps: map[string][]string{}, unresolved: map[string][]string{}, sccOf: map[string]int{}}
	for _, p := range reached {
		if ip := m.index.pkgs[p]; ip != nil && !ip.std {
			g.nodes = append(g.nodes, p)
		}
	}
	for _, p := range g.nodes {
		imps, err := m.importsOf(p, true)
		if err != nil {
			return nil, err
		}
		for _, imp := range imps {
			switch ip := m.index.pkgs[imp]; {
			case ip == nil:
				g.unresolved[p] = append(g.unresolved[p], imp)
			case !ip.std:
				g.deps[p] = append(g.deps[p], imp)
			}
		}
	}
	g.sccs = packageSCCs(g.nodes, g.deps)
	for i, comp := range g.sccs {
		for _, p := range comp {
			g.sccOf[p] = i
		}
	}
	return g, nil
}

// The compiler identity and runtime ABI are reached through variables so a test can move them.
var (
	compilerIdentity = version.Current
	runtimeAbi       = defaultRuntimeAbi
	executablePath   = os.Executable
)

func defaultRuntimeAbi() string { return llgen.CurrentAbiFingerprint(version.AbiRevision, nil) }

// cacheEnv hashes everything outside the sources that a cached result depends on: compiler build, target and profile, availability limits, and the runtime ABI.
func (m *Module) cacheEnv() string {
	h := sha256.New()
	p := compilerIdentity()
	fmt.Fprintf(h, "key%d|%s|%s|%v|%s|", keyVersion, p.Release, p.Commit, p.Dirty, p.GoVersion)
	if exe, err := executablePath(); err == nil {
		if st, err := os.Stat(exe); err == nil {
			fmt.Fprintf(h, "exe=%d,%d|", st.Size(), st.ModTime().UnixNano())
		}
	}
	fmt.Fprintf(h, "%s|%+v|", targetIdentity(m.target), m.target)
	fmt.Fprintf(h, "deny=%s|ceil=%s|art=%s|test=%v|verify=%v|", strings.Join(slices.Sorted(slices.Values(m.deny)), ","), m.ceiling, m.artifactName, m.test, m.verifyPasses)
	fmt.Fprintf(h, "isr=%s|%s|", m.Manifest.Interrupt.Canonical(), mir.IsrContracts.IsrVersion())
	fmt.Fprintf(h, "abi=%s", runtimeAbi())
	return hex.EncodeToString(h.Sum(nil))
}

// stdDigest hashes every std file: user code reaches std implicitly, so any std edit invalidates every package key.
func (m *Module) stdDigest() (string, error) {
	h := sha256.New()
	for _, path := range m.index.order {
		ip := m.index.pkgs[path]
		if !ip.std {
			continue
		}
		if err := m.writeFiles(h, ip); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m *Module) writeFiles(h hash.Hash, ip *indexedPackage) error {
	for _, f := range ip.files {
		if err := m.scan(f); err != nil {
			return err
		}
		fmt.Fprintf(h, "file=%s:%s\n", f.display, f.hash)
	}
	return nil
}

// packageKey addresses a record by the sources of path and its component, the artifacts it imports and the environment.
func (m *Module) packageKey(g *pkgGraph, path, env, std string, digests map[string]string) (string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "env=%s\nstd=%s\npkg=%s\n", env, std, path)
	comp := g.sccs[g.sccOf[path]]
	for _, member := range comp {
		fmt.Fprintf(h, "member=%s\n", member)
		if err := m.writeFiles(h, m.index.pkgs[member]); err != nil {
			return "", err
		}
		if m.keyExplicit && m.holdsFile(member, m.keyEntry) {
			fmt.Fprintf(h, "entry=%s\n", m.keyEntry)
		}
	}
	for _, dep := range g.closure(path) {
		fmt.Fprintf(h, "dep=%s:%s\n", dep, digests[dep])
	}
	for _, member := range comp {
		for _, miss := range g.unresolved[member] {
			fmt.Fprintf(h, "unresolved=%s\n", miss)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// holdsFile says whether the indexed package has the file with this display name; the explicit entry is the one script file its package may hold outside cmd/.
func (m *Module) holdsFile(path, display string) bool {
	return slices.ContainsFunc(m.index.pkgs[path].files, func(f *indexedFile) bool { return f.display == display })
}

// closure lists every package path reaches through imports: a distant public type can decide a layout that flows through an intermediate package.
func (g *pkgGraph) closure(path string) []string {
	seen := map[string]bool{}
	queue := slices.Clone(g.sccs[g.sccOf[path]])
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, dep := range g.deps[cur] {
			if g.sccOf[dep] == g.sccOf[path] || seen[dep] {
				continue
			}
			seen[dep] = true
			queue = append(queue, dep)
		}
	}
	return slices.Sorted(maps.Keys(seen))
}
