package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type fieldSiteKey struct {
	Key fieldFnKey
	Fn  EntityID
}

type edgeSiteKey struct {
	File   FileID
	Node   syntax.NodeID
	Target EntityID
}

type fnSiteSrc struct {
	from    fieldFnKey
	hasFrom bool
	file    FileID
	node    syntax.NodeID
}

func (r *Result) joinKnownFieldFnAt(key fieldFnKey, add []EntityID, s fnSiteSrc) bool {
	changed := false
	for _, f := range add {
		if slices.Contains(r.knownFieldFn[key], f) {
			continue
		}
		r.knownFieldFn[key] = append(r.knownFieldFn[key], f)
		changed = true
		r.noteFnSite(key, f, s)
	}
	return changed
}

func (r *Result) noteFnSite(key fieldFnKey, f EntityID, s fnSiteSrc) {
	k := fieldSiteKey{Key: key, Fn: f}
	if _, ok := r.fieldFnSites[k]; ok {
		return
	}
	site := fieldPoisonSiteInfo{File: s.file, Node: s.node}
	if s.hasFrom {
		if from, ok := r.fieldFnSites[fieldSiteKey{Key: s.from, Fn: f}]; ok {
			site = from
		}
	}
	if site.File == 0 {
		return
	}
	if r.fieldFnSites == nil {
		r.fieldFnSites = map[fieldSiteKey]fieldPoisonSiteInfo{}
	}
	r.fieldFnSites[k] = site
}

func (r *Result) noteEdgeSite(key fieldFnKey, file FileID, node syntax.NodeID, tgt EntityID) {
	site, ok := r.fieldFnSites[fieldSiteKey{Key: key, Fn: tgt}]
	if !ok {
		return
	}
	if r.edgeFnSites == nil {
		r.edgeFnSites = map[edgeSiteKey]fieldPoisonSiteInfo{}
	}
	k := edgeSiteKey{File: file, Node: node, Target: tgt}
	if _, seen := r.edgeFnSites[k]; !seen {
		r.edgeFnSites[k] = site
	}
}

func (r *Result) fieldWriteHop(e EffectEdge) (OpHop, bool) {
	site, ok := r.edgeFnSites[edgeSiteKey{File: e.File, Node: e.Node, Target: e.Target}]
	if !ok {
		return OpHop{}, false
	}
	name := "`" + r.Entities[e.Target].Name + "`"
	if r.Entities[e.Target].Kind == EntClosure {
		name = "this closure"
	}
	return OpHop{File: site.File, Node: site.Node, Text: name + " is stored in the field here"}, true
}

func (r *Result) noteEdgeSiteAt(file FileID, node syntax.NodeID, tgt EntityID, wfile FileID, wnode syntax.NodeID) {
	if r.edgeFnSites == nil {
		r.edgeFnSites = map[edgeSiteKey]fieldPoisonSiteInfo{}
	}
	k := edgeSiteKey{File: file, Node: node, Target: tgt}
	if _, seen := r.edgeFnSites[k]; !seen {
		r.edgeFnSites[k] = fieldPoisonSiteInfo{File: wfile, Node: wnode}
	}
}
