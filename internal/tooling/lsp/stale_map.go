package lsp

import (
	"bytes"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/lang/token"
)

type textShift struct {
	segs           []shiftSeg
	oldLen, curLen int
}

type shiftSeg struct {
	oldStart, oldEnd, curStart, curEnd int
	same                               bool
}

func newTextShift(old, cur []byte) textShift {
	a, b := splitLines(old), splitLines(cur)
	ao, bo := lineOffsets(a), lineOffsets(b)
	sh := textShift{oldLen: len(old), curLen: len(cur)}
	pa, pb := 0, 0
	for _, h := range diffLines(a, b) {
		sh.add(shiftSeg{pa, ao[h.a0], pb, bo[h.b0], true})
		sh.addEdit(old, cur, shiftSeg{ao[h.a0], ao[h.a1], bo[h.b0], bo[h.b1], false})
		pa, pb = ao[h.a1], bo[h.b1]
	}
	sh.add(shiftSeg{pa, len(old), pb, len(cur), true})
	return sh
}

func lineOffsets(lines [][]byte) []int {
	offs := make([]int, len(lines)+1)
	for i, l := range lines {
		offs[i+1] = offs[i] + len(l)
	}
	return offs
}

func (sh *textShift) add(s shiftSeg) {
	if s.oldEnd > s.oldStart || s.curEnd > s.curStart {
		sh.segs = append(sh.segs, s)
	}
}

func (sh *textShift) addEdit(old, cur []byte, s shiftSeg) {
	o, c := old[s.oldStart:s.oldEnd], cur[s.curStart:s.curEnd]
	p := 0
	for p < min(len(o), len(c)) && o[p] == c[p] {
		p++
	}
	q := 0
	for q < min(len(o), len(c))-p && o[len(o)-1-q] == c[len(c)-1-q] {
		q++
	}
	sh.add(shiftSeg{s.oldStart, s.oldStart + p, s.curStart, s.curStart + p, true})
	sh.segs = append(sh.segs, shiftSeg{s.oldStart + p, s.oldEnd - q, s.curStart + p, s.curEnd - q, false})
	sh.add(shiftSeg{s.oldEnd - q, s.oldEnd, s.curEnd - q, s.curEnd, true})
}

// toOld maps an offset of the current text to the snapshot text; false when it lies in an edited region.
func (sh textShift) toOld(pos token.Pos) (token.Pos, bool) {
	i := sort.Search(len(sh.segs), func(i int) bool { return sh.segs[i].curEnd >= int(pos) })
	for ; i < len(sh.segs) && sh.segs[i].curStart <= int(pos); i++ {
		if s := sh.segs[i]; s.same {
			return pos - token.Pos(s.curStart) + token.Pos(s.oldStart), true
		}
	}
	return 0, false
}

// toCur maps a snapshot offset to the current text; an offset inside an edited region lands at its start.
func (sh textShift) toCur(pos token.Pos) token.Pos {
	if i := sh.at(pos); i >= 0 {
		if s := sh.segs[i]; s.same {
			return pos - token.Pos(s.oldStart) + token.Pos(s.curStart)
		}
		return token.Pos(sh.segs[i].curStart)
	}
	return pos
}

func (sh textShift) at(pos token.Pos) int {
	i := sort.Search(len(sh.segs), func(i int) bool { return sh.segs[i].oldEnd >= int(pos) })
	hit := -1
	for ; i < len(sh.segs) && sh.segs[i].oldStart <= int(pos); i++ {
		if sh.segs[i].same {
			return i
		}
		if hit < 0 && int(pos) > sh.segs[i].oldStart && int(pos) < sh.segs[i].oldEnd {
			hit = i
		}
	}
	return hit
}

// toCurSpan maps a snapshot span to the current text; an end inside an edited region widens to that region's whole lines, and a region that no longer exists collapses the span to the point where it was.
func (sh textShift) toCurSpan(cur []byte, sp token.Span) token.Span {
	out := token.Span{Start: sh.toCur(sp.Start), End: sh.toCur(sp.End)}
	si, ei := sh.at(sp.Start), sh.at(sp.End)
	for _, i := range []int{si, ei} {
		if i >= 0 && !sh.segs[i].same && sh.segs[i].curStart == sh.segs[i].curEnd {
			p := token.Pos(sh.segs[i].curStart)
			return token.Span{Start: p, End: p}
		}
	}
	if si >= 0 && !sh.segs[si].same {
		out.Start = token.Pos(bytes.LastIndexByte(cur[:sh.segs[si].curStart], '\n') + 1)
	}
	if ei >= 0 && !sh.segs[ei].same {
		end := sh.segs[ei].curEnd
		if n := bytes.IndexByte(cur[end:], '\n'); n >= 0 {
			end += n
		} else {
			end = len(cur)
		}
		out.End = token.Pos(end)
	}
	return out
}

type shiftEntry struct {
	f    *token.File
	text string
	cur  *token.File
	sh   textShift
	same bool
}

// docShift is the shift between snapshot file f and the open document it was read from; ok is false when they are the same text.
func (s *Server) docShift(snap *snapshot, f *token.File) (cur *token.File, sh textShift, ok bool) {
	key := uri.File(s.pathOfFile(snap, f.Name))
	text, open := s.docs[key]
	if !open {
		return nil, textShift{}, false
	}
	s.shiftMu.Lock()
	defer s.shiftMu.Unlock()
	e := s.shifts[key]
	if e == nil || e.f != f || e.text != text {
		e = &shiftEntry{f: f, text: text, same: text == string(f.Src)}
		if !e.same {
			e.cur = fileOfText(text)
			e.sh = newTextShift(f.Src, e.cur.Src)
		}
		if s.shifts == nil {
			s.shifts = map[uri.URI]*shiftEntry{}
		}
		s.shifts[key] = e
	}
	return e.cur, e.sh, !e.same
}

// rng is spanRange for an answer taken from a snapshot: when the document has been edited since, the range is moved to where the same text now is.
func (s *Server) rng(snap *snapshot, f *token.File, sp token.Span) protocol.Range {
	cur, sh, ok := s.docShift(snap, f)
	if !ok {
		return spanRange(f, sp)
	}
	return spanRange(cur, sh.toCurSpan(cur.Src, sp))
}

// offsetIn converts a client position to an offset of snapshot file f, false when the text there changed since the snapshot.
func (s *Server) offsetIn(snap *snapshot, f *token.File, p protocol.Position) (token.Pos, bool) {
	cur, sh, ok := s.docShift(snap, f)
	if !ok {
		return offset(f, p), true
	}
	return sh.toOld(offset(cur, p))
}
