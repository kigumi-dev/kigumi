package sem

import (
	"slices"
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// PackageIdentity is a package's content-derived identity, independent of the
// ordinal PackageID the checker uses to index the result.
type PackageIdentity struct {
	Module string
	Path   string
	ID     syntax.PackageID
}

// AnalysisSnapshot is the immutable view of a published Result: content-derived
// identities plus the diagnostic set the phase produced. It exposes no setter
// and hands out copies, so a consumer cannot mutate the published analysis.
type AnalysisSnapshot struct {
	module   syntax.ModuleID
	packages []PackageIdentity
	diags    []diag.Diagnostic
	rendered string
	errors   bool
}

// ModuleID returns the content-derived identity of the checked module.
func (s *AnalysisSnapshot) ModuleID() syntax.ModuleID {
	if s == nil {
		return ""
	}
	return s.module
}

// Packages lists the checked packages in module/path order.
func (s *AnalysisSnapshot) Packages() []PackageIdentity {
	if s == nil {
		return nil
	}
	return append([]PackageIdentity(nil), s.packages...)
}

// PackageID returns the content-derived id of path; false when not checked.
func (s *AnalysisSnapshot) PackageID(path string) (syntax.PackageID, bool) {
	if s == nil {
		return "", false
	}
	for _, p := range s.packages {
		if p.Path == path {
			return p.ID, true
		}
	}
	return "", false
}

// Diagnostics returns a copy of every diagnostic Render prints.
func (s *AnalysisSnapshot) Diagnostics() []diag.Diagnostic {
	if s == nil {
		return nil
	}
	return cloneDiagnostics(s.diags)
}

// HasErrors reports whether the published check carries an error.
func (s *AnalysisSnapshot) HasErrors() bool {
	return s != nil && s.errors
}

// Render returns the diagnostics as text; "" when clean.
func (s *AnalysisSnapshot) Render() string {
	if s == nil {
		return ""
	}
	return s.rendered
}

// Snapshot publishes r: it freezes the result and returns the immutable view.
// It is idempotent, so a later phase may publish the same result again only by
// reading the snapshot that already exists.
func (r *Result) Snapshot() *AnalysisSnapshot {
	if r == nil {
		return nil
	}
	if r.snapshot != nil {
		return r.snapshot
	}
	r.sealed = true
	s := &AnalysisSnapshot{module: r.ModuleID(), diags: cloneDiagnostics(r.DiagnosticsAll()), rendered: r.Render(), errors: r.HasErrors()}
	for id := 1; id < len(r.Packages); id++ {
		p := &r.Packages[id]
		s.packages = append(s.packages, PackageIdentity{Module: p.Module, Path: p.Path, ID: r.packageIdentity(PackageID(id))})
	}
	slices.SortStableFunc(s.packages, func(a, b PackageIdentity) int {
		if c := strings.Compare(a.Module, b.Module); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	r.snapshot = s
	return s
}

// Sealed reports that the result has been published and no longer accepts
// builder diagnostics.
func (r *Result) Sealed() bool { return r != nil && r.sealed }

// AppendDiag adds a diagnostic a phase after the checker produced (MIR
// lowering, comptime folding). It is a builder operation: publishing the
// result seals it, and a later append is a compiler bug, not a diagnostic.
func (r *Result) AppendDiag(file FileID, d diag.Diagnostic) {
	if r.sealed {
		panic("sem: AppendDiag after the Result was published")
	}
	if int(file) >= len(r.Files) {
		return
	}
	r.Files[file].Diags = append(r.Files[file].Diags, d)
}

// AppendTreeDiag adds a phase diagnostic to the file tree t owns.
func (r *Result) AppendTreeDiag(t *syntax.Tree, d diag.Diagnostic) {
	if r.sealed {
		panic("sem: AppendTreeDiag after the Result was published")
	}
	if f := r.File(t); f != nil {
		f.Diags = append(f.Diags, d)
	}
}

// AppendDiags adds several phase diagnostics to file, with the same
// publish guard as AppendDiag.
func (r *Result) AppendDiags(file FileID, ds []diag.Diagnostic) {
	if r.sealed {
		panic("sem: AppendDiags after the Result was published")
	}
	if int(file) >= len(r.Files) {
		return
	}
	r.Files[file].Diags = append(r.Files[file].Diags, ds...)
}

// ModuleID returns the content-derived identity of the checked module.
func (r *Result) ModuleID() syntax.ModuleID {
	ids := make([]syntax.PackageID, 0, len(r.Packages))
	for id := 1; id < len(r.Packages); id++ {
		ids = append(ids, r.packageIdentity(PackageID(id)))
	}
	return syntax.ModuleIdentity(r.moduleName(), ids)
}

// PackageKey returns the content-derived identity of pkg; "" for the
// universe pseudo-package (ordinal 0) or an out-of-range id.
func (r *Result) PackageKey(pkg PackageID) syntax.PackageID {
	if pkg == 0 || int(pkg) >= len(r.Packages) {
		return ""
	}
	return r.packageIdentity(pkg)
}

func (r *Result) packageIdentity(pkg PackageID) syntax.PackageID {
	p := &r.Packages[pkg]
	files := make([]*token.File, 0, len(p.Files))
	for _, fid := range p.Files {
		if int(fid) < len(r.Files) && r.Files[fid].Tree != nil && r.Files[fid].Tree.File != nil {
			files = append(files, r.Files[fid].Tree.File)
		}
	}
	return syntax.PackageIdentity(p.Module, p.Path, files)
}

func (r *Result) moduleName() string {
	if r.mod == nil {
		return ""
	}
	return r.mod.Name
}

// cloneDiagnostics copies the whole tree, so a reader holding one diagnostic
// cannot reach a slice the published snapshot still owns.
func cloneDiagnostics(in []diag.Diagnostic) []diag.Diagnostic {
	if in == nil {
		return nil
	}
	out := make([]diag.Diagnostic, len(in))
	for i, d := range in {
		out[i] = d
		out[i].Notes = cloneDiagnostics(d.Notes)
		out[i].Help = append([]string(nil), d.Help...)
		out[i].Fixes = append([]diag.Fix(nil), d.Fixes...)
	}
	return out
}
