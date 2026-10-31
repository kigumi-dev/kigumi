package lsp

import (
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// workspaceEdit uses versioned documentChanges when the client declared
// support, so a stale edit is rejected rather than applied over newer content.
func (s *Server) workspaceEdit(changes map[uri.URI][]protocol.TextEdit) *protocol.WorkspaceEdit {
	if !s.documentChanges {
		return &protocol.WorkspaceEdit{Changes: changes}
	}
	uris := make([]uri.URI, 0, len(changes))
	for u := range changes {
		uris = append(uris, u)
	}
	sort.Slice(uris, func(i, j int) bool { return uris[i] < uris[j] })
	docChanges := make([]protocol.DocumentChange, 0, len(uris))
	for _, u := range uris {
		edits := changes[u]
		elems := make([]protocol.TextDocumentEditElement, len(edits))
		for i := range edits {
			e := edits[i]
			elems[i] = &e
		}
		var version *int32
		if v, ok := s.docVersions[u]; ok {
			version = &v
		}
		docChanges = append(docChanges, &protocol.TextDocumentEdit{
			TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: u},
				Version:                version,
			},
			Edits: elems,
		})
	}
	return &protocol.WorkspaceEdit{DocumentChanges: docChanges}
}
