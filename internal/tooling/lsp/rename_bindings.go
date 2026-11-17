package lsp

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/base/errors"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/driver"
)

type renameSpan struct {
	start, end token.Pos
	text       string
	delta      int
}

type renameSite struct {
	file string
	pos  token.Pos
}

func (s *Server) checkRenameBindings(ctx context.Context, snap *snapshot, changes map[uri.URI][]protocol.TextEdit) error {
	overlay := make(map[string][]byte, len(s.docs)+len(changes))
	for u, text := range s.docs {
		overlay[uriToPath(u)] = []byte(text)
	}
	spans := map[string][]renameSpan{}
	for u, edits := range changes {
		path := uriToPath(u)
		tree := snap.fileOf(path)
		if tree == nil {
			return errors.New("cannot check rename bindings without the source file")
		}
		var list []renameSpan
		for _, edit := range edits {
			list = append(list, renameSpan{start: offset(tree.File, edit.Range.Start), end: offset(tree.File, edit.Range.End), text: edit.NewText})
		}
		slices.SortFunc(list, func(a, b renameSpan) int { return int(a.start) - int(b.start) })
		delta := 0
		for i := range list {
			delta += len(list[i].text) - int(list[i].end-list[i].start)
			list[i].delta = delta
		}
		spans[tree.File.Name] = list
		source := make([]byte, 0, len(tree.File.Src))
		var at token.Pos
		for _, edit := range list {
			source = append(source, tree.File.Src[at:edit.start]...)
			source = append(source, edit.text...)
			at = edit.end
		}
		overlay[path] = append(source, tree.File.Src[at:]...)
	}
	module, err := driver.LoadModule(snap.root, driver.LoadOptions{
		Test: true, StdRoot: s.stdRoot, Entry: snap.mod.Entry, Overlay: overlay,
		Target: s.target, AllPackages: true, PackageCache: s.cache, Compiler: s.compiler, LazyStd: s.lazyStd,
	})
	if err != nil {
		return errors.NewErrf("cannot check rename bindings: %v", err)
	}
	next, err := driver.CheckForEditorContext(ctx, module)
	if err != nil {
		return errors.NewErrf("cannot check rename bindings: %v", err)
	}
	if next.HasErrors() && !snap.res.HasErrors() {
		return errors.New("rename would introduce a compiler error")
	}
	resolved := map[renameSite]string{}
	for i := 1; i < len(next.Files); i++ {
		file := &next.Files[i]
		if _, changed := spans[file.Tree.File.Name]; !changed {
			continue
		}
		for n, ent := range file.Uses {
			if ent != 0 {
				sp := nameSpan(file.Tree, syntax.NodeID(n))
				resolved[renameSite{file.Tree.File.Name, sp.Start}] = renameEntityKey(next, ent, nil)
			}
		}
	}
	for i := 1; i < len(snap.res.Files); i++ {
		file := &snap.res.Files[i]
		list, changed := spans[file.Tree.File.Name]
		if !changed {
			continue
		}
		for n, ent := range file.Uses {
			if ent == 0 {
				continue
			}
			sp := nameSpan(file.Tree, syntax.NodeID(n))
			site := renameSite{file.Tree.File.Name, renamedOffset(sp.Start, list)}
			if resolved[site] != renameEntityKey(snap.res, ent, spans) {
				return errors.New("rename would change which declaration a reference resolves to")
			}
		}
	}
	return nil
}

func renamedOffset(pos token.Pos, edits []renameSpan) token.Pos {
	i := sort.Search(len(edits), func(i int) bool { return edits[i].end > pos })
	if i == 0 {
		return pos
	}
	return token.Pos(int(pos) + edits[i-1].delta)
}

func renameEntityKey(res *sem.Result, ent sem.EntityID, edits map[string][]renameSpan) string {
	e := res.Entity(ent)
	if e.File != 0 && e.Tok != 0 {
		tree := res.Tree(e.File)
		pos := renamedOffset(tree.Toks[e.Tok].Span.Start, edits[tree.File.Name])
		return fmt.Sprint(tree.File.Name, ":", pos, ":", e.Kind)
	}
	return fmt.Sprint(e.Kind, ":", e.Pkg, ":", e.Name)
}
