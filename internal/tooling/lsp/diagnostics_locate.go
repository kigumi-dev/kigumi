package lsp

import (
	"path/filepath"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/driver"
)

// locatedDiag is a diagnostic filed under the file its location is in,
// which is not always the file whose list it came from.
type locatedDiag struct {
	file *token.File
	d    diag.Diagnostic
}

// locate resolves the file a diagnostic points into; a file without a real path stays on owner at offset 0.
func (snap *snapshot) locate(owner *token.File, d diag.Diagnostic) locatedDiag {
	if snap.res == nil || d.Loc.Source == 0 || d.Loc.Source == owner.ID {
		return locatedDiag{owner, d}
	}
	nf := snap.res.Source(d.Loc.Source)
	if nf == nil || nf.Generated {
		d.Loc.Span = token.Span{}
		return locatedDiag{owner, d}
	}
	if _, ok := snap.originOf(nf.Name); !ok {
		d.Loc.Span = token.Span{}
		return locatedDiag{owner, d}
	}
	return locatedDiag{nf, d}
}

// diagnosticsOf lists the diagnostics that point into the file named name,
// whichever file's list they were reported under.
func (snap *snapshot) diagnosticsOf(name string) []diag.Diagnostic {
	snap.diagOnce.Do(func() {
		snap.diags = map[string][]diag.Diagnostic{}
		for _, p := range snap.mod.Order {
			for _, t := range snap.mod.Packages[p].Files {
				all := append([]diag.Diagnostic{}, t.Diags...)
				if snap.res != nil {
					all = append(all, snap.res.Diagnostics(t)...)
				}
				for _, d := range all {
					l := snap.locate(t.File, d)
					snap.diags[l.file.Name] = append(snap.diags[l.file.Name], l.d)
				}
			}
		}
	})
	return snap.diags[name]
}

type urlDiagnostics struct {
	order []uri.URI
	diags map[uri.URI][]protocol.Diagnostic
}

// diagnosticsByURI groups LSP diagnostics by the file they point into; every own file gets an entry.
func (s *Server) diagnosticsByURI(snap *snapshot) urlDiagnostics {
	out := urlDiagnostics{diags: map[uri.URI][]protocol.Diagnostic{}}
	add := func(u uri.URI) {
		if _, ok := out.diags[u]; !ok {
			out.diags[u] = nil
			out.order = append(out.order, u)
		}
	}
	for _, p := range snap.mod.Order {
		pkg := snap.mod.Packages[p]
		if pkg.Std {
			continue
		}
		for _, t := range pkg.Files {
			add(uri.File(s.pathOfFile(snap, t.File.Name)))
		}
	}
	for _, p := range snap.mod.Order {
		pkg := snap.mod.Packages[p]
		if pkg.Std {
			continue
		}
		for _, t := range pkg.Files {
			all := append([]diag.Diagnostic{}, t.Diags...)
			if snap.res != nil {
				all = append(all, snap.res.Diagnostics(t)...)
			}
			for _, d := range all {
				l := snap.locate(t.File, d)
				u := uri.File(s.pathOfFile(snap, l.file.Name))
				add(u)
				out.diags[u] = append(out.diags[u], s.toDiagnostic(snap, l.file, l.d))
			}
		}
	}
	if snap.build != nil {
		u := uri.File(filepath.Join(snap.root, driver.BuildFileName))
		add(u)
		f := snap.build.Tree.File
		for _, d := range snap.build.Diagnostics() {
			out.diags[u] = append(out.diags[u], buildDiagnostic(f, d))
		}
	}
	return out
}

// buildDiagnostic converts a build.kg diagnostic; related information stays
// inside build.kg, since the module snapshot's source store does not know the
// build program's own parse.
func buildDiagnostic(f *token.File, d diag.Diagnostic) protocol.Diagnostic {
	msg := d.Msg
	for _, h := range d.Help {
		msg += "\nhelp: " + h
	}
	out := protocol.Diagnostic{Range: spanRange(f, d.Loc.Span), Severity: protocol.DiagnosticSeverity(d.Severity + 1), Source: protocol.NewOptional("kigumi"), Message: protocol.String(msg)}
	if d.Code != "" {
		out.Code = protocol.String(d.Code)
	}
	for _, n := range d.Notes {
		if n.Loc.Source != 0 && n.Loc.Source != f.ID {
			continue
		}
		out.RelatedInformation = append(out.RelatedInformation, protocol.DiagnosticRelatedInformation{
			Location: protocol.Location{URI: uri.File(f.Name), Range: spanRange(f, n.Loc.Span)},
			Message:  n.Msg,
		})
	}
	return out
}
