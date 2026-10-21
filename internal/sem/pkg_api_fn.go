package sem

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"kigumi/internal/diag"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (r *Result) apiFn(d *APIDecl, id EntityID) {
	info := r.Fn(id)
	d.Sig = r.TypeString(info.Sig)
	d.Generics = append(r.apiParams(info.TypeParams), r.apiParams(info.RecvParams)...)
	for _, p := range info.Params {
		pe := &r.Entities[p]
		d.Params = append(d.Params, fmt.Sprintf("%s %s %d%s", pe.Name, r.TypeString(pe.Type), pe.Flags&(EfMut|EfMove|EfEscaping|EfVariadic), r.defaultFingerprint(p)))
	}
	d.Effects = fmt.Sprintf("%s pure=%v noalloc=%v default=%v known=%v recv=%d", effectBits(info.Declared), info.Summary.Pure, info.Summary.Noalloc, info.Summary.Default, info.Summary.Known, info.Recv)
	ops := r.opSummaryArtifact(id)
	d.Ops = &ops
	for i, p := range info.Params {
		for _, c := range info.CalledParams {
			if c == p {
				d.Called = append(d.Called, i)
			}
		}
	}
	for _, k := range info.CtxRequires {
		d.Ctx = append(d.Ctx, r.apiName(k))
	}
	d.Abi, d.Export, d.LinkName, d.Section = info.Abi, info.Export, info.LinkName, info.Section
	if r.genericFn(info) {
		d.Body = r.bodyFingerprint(id)
	}
}

func (r *Result) genericFn(info *FnInfo) bool {
	if len(info.TypeParams) > 0 || len(info.RecvParams) > 0 || len(info.ConstParams) > 0 {
		return true
	}
	return info.Owner != 0 && len(r.typeDecl(info.Owner).Params) > 0
}

// bodyFingerprint hashes the body's tokens without comments, so reformatting a generic body does not move the API.
func (r *Result) bodyFingerprint(fn EntityID) string {
	e := &r.Entities[fn]
	info := r.Fn(fn)
	if info.Body == 0 {
		return ""
	}
	t := r.tree(e.File)
	return tokenFingerprint(t, t.Span(info.Body))
}

func tokenFingerprint(t *syntax.Tree, span token.Span) string {
	h := sha256.New()
	src := t.File.Src[span.Start:span.End]
	for _, tk := range syntax.Lex(src, &diag.Bag{}) {
		if tk.Kind == token.Comment || tk.Kind == token.Newline {
			continue
		}
		fmt.Fprintf(h, "%d:%s ", tk.Kind, tk.Text(src))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
