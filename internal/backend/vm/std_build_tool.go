package vm

import (
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/buildgraph"
)

func (m *Machine) registerBuildFiles() {
	h := m.host
	str := func(v *obj) string { return string(deref(v).str()) }
	h["build.Builder.file"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		g := m.graphOf(a[0])
		path := str(a[1])
		abs := filepath.Join(g.Roots[0], filepath.FromSlash(path))
		if !withinRoots(g.Roots, abs) {
			return m.errMsg("file " + path + " is outside the module, its dependencies and the build output directory")
		}
		if _, err := os.Stat(abs); err != nil {
			return m.errMsg("file " + path + " does not exist under the module root")
		}
		g.Files = append(g.Files, buildgraph.FileRef{ID: len(g.Files), Kind: "literal", Path: filepath.ToSlash(path), Provenance: m.provenance()})
		return m.ok(mkOpaque("build.FilePath", len(g.Files)-1))
	}
	h["build.Builder.write"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		g := m.graphOf(a[0])
		name := str(a[1])
		if !bareOutputName(name) {
			return m.errMsg("write needs a bare file name, not " + name)
		}
		if other, taken := claimOutputName(g, name); taken {
			return m.errMsg("write " + name + " collides with " + other)
		}
		g.Files = append(g.Files, buildgraph.FileRef{ID: len(g.Files), Kind: "generated", Path: name, Content: str(a[2]), Provenance: m.provenance()})
		return m.ok(mkOpaque("build.FilePath", len(g.Files)-1))
	}
	h["build.Builder.runTool"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		g := m.graphOf(a[0])
		step := buildgraph.ToolStep{ID: len(g.ToolSteps), Framework: m.handleOf(a[1], "build.Framework"), Tool: str(a[2]), Provenance: m.provenance()}
		for _, e := range deref(a[3]).fields {
			step.Args = append(step.Args, str(e))
		}
		for _, e := range deref(a[4]).fields {
			step.Inputs = append(step.Inputs, m.handleOf(e, "build.FilePath"))
		}
		outs, ids, errObj := m.recordToolOutputs(g, step.ID, a[5])
		if errObj != nil {
			return errObj
		}
		step.Outputs = ids
		for _, e := range deref(a[6]).fields {
			step.Secrets = append(step.Secrets, m.handleOf(e, "build.Secret"))
		}
		g.ToolSteps = append(g.ToolSteps, step)
		return m.ok(mkArray(outs))
	}
	h["build.Builder.runArtifact"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		g := m.graphOf(a[0])
		step := buildgraph.ToolStep{ID: len(g.ToolSteps), Kind: "artifact", Artifact: m.handleOf(a[1], "build.Artifact"), Provenance: m.provenance()}
		for _, e := range deref(a[2]).fields {
			step.Args = append(step.Args, str(e))
		}
		for _, e := range deref(a[3]).fields {
			step.Inputs = append(step.Inputs, m.handleOf(e, "build.FilePath"))
		}
		outs, ids, errObj := m.recordToolOutputs(g, step.ID, a[4])
		if errObj != nil {
			return errObj
		}
		step.Outputs = ids
		g.ToolSteps = append(g.ToolSteps, step)
		return m.ok(mkArray(outs))
	}
}

// recordToolOutputs registers each declared output name as a new file
// produced by stepID, shared by runTool and runArtifact.
func (m *Machine) recordToolOutputs(g *buildgraph.Graph, stepID int, names *obj) (outs []*obj, ids []int, errObj *obj) {
	for _, e := range deref(names).fields {
		name := string(deref(e).str())
		if !bareOutputName(name) {
			return nil, nil, m.errMsg("tool outputs are bare file names, not " + name)
		}
		if other, taken := claimOutputName(g, name); taken {
			return nil, nil, m.errMsg("output " + name + " collides with " + other)
		}
		g.Files = append(g.Files, buildgraph.FileRef{ID: len(g.Files), Kind: "toolOutput", Path: name, ProducedBy: stepID, Provenance: m.provenance()})
		ids = append(ids, len(g.Files)-1)
		outs = append(outs, mkOpaque("build.FilePath", len(g.Files)-1))
	}
	if ids == nil {
		ids = []int{}
	}
	return outs, ids, nil
}

// withinRoots accepts an absolute path under a sandboxed root after resolving symlinks on both sides, so a planted link can't point outside it; a nonexistent path is judged by its nearest existing ancestor.
func withinRoots(roots []string, path string) bool {
	path = resolveExisting(filepath.Clean(path))
	for _, r := range roots {
		r = resolveExisting(r)
		rel, err := filepath.Rel(r, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func resolveExisting(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	dir, base := filepath.Split(filepath.Clean(path))
	if dir == "" || dir == path {
		return path
	}
	return filepath.Join(resolveExisting(filepath.Clean(dir)), base)
}
