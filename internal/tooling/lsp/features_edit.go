package lsp

import (
	"context"
	"fmt"
	"slices"
	"unicode"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/base/errors"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// isIdentifier follows the lexer's XID-like rule: a letter or `_` starts,
// letters, marks, digits and `_` continue.
func isIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !(r == '_' || unicode.IsLetter(r)) {
				return false
			}
			continue
		}
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)) {
			return false
		}
	}
	return !token.Lookup(name).IsKeyword()
}

// PrepareRename answers with the range of the name under the cursor when it
// belongs to a declaration of this module.
func (s *Server) PrepareRename(ctx context.Context, p *protocol.PrepareRenameParams) (protocol.PrepareRenameResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, t, n, ent := s.entityAt(ctx, p.TextDocumentPositionParams, true)
	if t == nil || !renamable(snap, ent) {
		return nil, nil
	}
	sp := t.Toks[snap.res.Entity(ent).Tok].Span
	if n != 0 {
		sp = nameSpan(t, n)
	}
	r := spanRange(t.File, sp)
	return &r, nil
}

// Rename changes the declaration and every use of an entity, and keeps
// interface requirements and their witnesses in step.
func (s *Server) Rename(ctx context.Context, p *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !isIdentifier(p.NewName) {
		return nil, errors.NewErrf("%q is not a valid identifier", p.NewName)
	}
	snap, t, _, ent := s.entityAt(ctx, p.TextDocumentPositionParams, true)
	if t == nil || ent == 0 {
		return nil, nil
	}
	if !renamable(snap, ent) {
		return nil, errors.New("only declarations of this module can be renamed")
	}
	if collidesInPackage(snap.res, ent, p.NewName) {
		pkgPath := snap.res.Packages[snap.res.Entity(ent).Pkg].Path
		return nil, errors.NewErrf("%q already declares a member named %q", pkgPath, p.NewName)
	}
	targets := []sem.EntityID{ent}
	if isMethod(snap.res, ent) {
		for _, m := range snap.implementations(ent) {
			if renamable(snap, m) {
				targets = append(targets, m)
			}
		}
	}
	slices.Sort(targets)
	targets = slices.Compact(targets)
	changes := map[uri.URI][]protocol.TextEdit{}
	seen := map[string]bool{}
	add := func(f *token.File, sp token.Span) {
		key := fmt.Sprint(f.Name, sp)
		if seen[key] {
			return
		}
		seen[key] = true
		if o, ok := snap.originOf(f.Name); ok && !o.editable {
			s.logf(ctx, protocol.MessageTypeWarning, "rename: skipping non-editable %s source %s", o.kind, f.Name)
			return
		}
		u := uri.File(s.pathOfFile(snap, f.Name))
		changes[u] = append(changes[u], protocol.TextEdit{Range: spanRange(f, sp), NewText: p.NewName})
	}
	for _, id := range targets {
		e := snap.res.Entity(id)
		decl := snap.res.Tree(e.File)
		add(decl.File, decl.Toks[e.Tok].Span)
	}
	old := snap.res.Entity(ent).Name
	for _, u := range snap.semIndex().usesOf(targets) {
		sp := nameSpan(u.tree, u.node)
		if text := string(u.tree.File.Src[sp.Start:sp.End]); text != old && isIdentifier(text) {
			continue
		}
		add(u.tree.File, sp)
	}
	for _, id := range targets {
		for _, imp := range snap.semIndex().imports[id] {
			if f, sp, ok := importedName(snap.res, imp, old); ok {
				add(f, sp)
			}
		}
	}
	return s.workspaceEdit(changes), nil
}

// importedName is the span of the name an import binding takes from its
// package: the binding itself, or the name before `as` of a renamed import.
func importedName(res *sem.Result, imp sem.EntityID, name string) (*token.File, token.Span, bool) {
	e := res.Entity(imp)
	t := res.Tree(e.File)
	if e.Name == name {
		return t.File, t.Toks[e.Tok].Span, true
	}
	for v := range t.All() {
		if v.Kind() == syntax.ImportAlias && v.AsImportAlias().Alias().ID() == e.Node {
			orig := v.AsImportAlias().Name()
			return t.File, t.Span(orig.ID()), true
		}
	}
	return nil, token.Span{}, false
}

// renamable excludes std, predeclared, unnamed and any entity declared
// outside this module, since rename only updates this module's own usages.
func renamable(snap *snapshot, ent sem.EntityID) bool {
	if snap == nil || snap.res == nil || ent == 0 {
		return false
	}
	e := snap.res.Entity(ent)
	if e.File == 0 || e.Tok == 0 {
		return false
	}
	decl := snap.res.Tree(e.File)
	o, ok := snap.originOf(decl.File.Name)
	return ok && o.editable
}

// collidesInPackage preflights name collisions; methods are left unchecked
// since their sibling methods are an overload set, not a name collision.
func collidesInPackage(res *sem.Result, ent sem.EntityID, newName string) bool {
	e := res.Entity(ent)
	switch {
	case e.Kind == sem.EntFn && !isMethod(res, ent):
		if other := res.PackageMember(e.Pkg, newName); other != 0 && other != ent {
			return true
		}
	case e.Kind == sem.EntType || e.Kind == sem.EntInterface || e.Kind == sem.EntConst || e.Kind == sem.EntStatic || e.Kind == sem.EntAlias || e.Kind == sem.EntConstraint:
		if other := res.PackageMember(e.Pkg, newName); other != 0 && other != ent {
			return true
		}
	case e.Kind == sem.EntField:
		if other := res.FindField(e.Parent, newName); other != 0 && other != ent {
			return true
		}
	}
	return false
}
