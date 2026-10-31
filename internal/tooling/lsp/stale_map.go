package lsp

import (
	"bytes"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/lang/token"
)

// textShift relates byte offsets of a snapshot's text to the open
// document's text, assuming the edits since touch one contiguous region.
type textShift struct {
	prefix, suffix int
	oldLen, curLen int
}

func newTextShift(old, cur []byte) textShift {
	n := min(len(old), len(cur))
	p := 0
	for p < n && old[p] == cur[p] {
		p++
	}
	q := 0
	for q < n-p && old[len(old)-1-q] == cur[len(cur)-1-q] {
		q++
	}
	return textShift{prefix: p, suffix: q, oldLen: len(old), curLen: len(cur)}
}

// toOld maps an offset of the current text to the snapshot text; false when it lies in the edited region.
func (sh textShift) toOld(pos token.Pos) (token.Pos, bool) {
	switch {
	case int(pos) <= sh.prefix:
		return pos, true
	case int(pos) >= sh.curLen-sh.suffix:
		return pos - token.Pos(sh.curLen) + token.Pos(sh.oldLen), true
	}
	return 0, false
}

// toCur maps a snapshot offset to the current text; an offset inside the edited region lands at its start.
func (sh textShift) toCur(pos token.Pos) token.Pos {
	switch {
	case int(pos) <= sh.prefix:
		return pos
	case int(pos) >= sh.oldLen-sh.suffix:
		return pos + token.Pos(sh.curLen) - token.Pos(sh.oldLen)
	}
	return token.Pos(sh.prefix)
}

// docShift is the shift between snapshot file f and the open document it was read from; ok is false when they are the same text.
func (s *Server) docShift(snap *snapshot, f *token.File) (cur *token.File, sh textShift, ok bool) {
	text, open := s.docs[uri.File(s.pathOfFile(snap, f.Name))]
	if !open || bytes.Equal([]byte(text), f.Src) {
		return nil, textShift{}, false
	}
	return fileOfText(text), newTextShift(f.Src, []byte(text)), true
}

// rng is spanRange for an answer taken from a snapshot: when the document has been edited since, the range is moved to where the same text now is.
func (s *Server) rng(snap *snapshot, f *token.File, sp token.Span) protocol.Range {
	cur, sh, ok := s.docShift(snap, f)
	if !ok {
		return spanRange(f, sp)
	}
	return spanRange(cur, token.Span{Start: sh.toCur(sp.Start), End: sh.toCur(sp.End)})
}

// offsetIn converts a client position to an offset of snapshot file f, false when the text there changed since the snapshot.
func (s *Server) offsetIn(snap *snapshot, f *token.File, p protocol.Position) (token.Pos, bool) {
	cur, sh, ok := s.docShift(snap, f)
	if !ok {
		return offset(f, p), true
	}
	return sh.toOld(offset(cur, p))
}
