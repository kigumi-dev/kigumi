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
	Param  EntityID
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
		if !slices.Contains(r.knownFieldFn[key], f) {
			r.knownFieldFn[key] = append(r.knownFieldFn[key], f)
			changed = true
		}
		if !slices.Contains(r.flow.fnHist[key], f) {
			if r.flow.fnHist == nil {
				r.flow.fnHist = map[fieldFnKey][]EntityID{}
			}
			r.flow.fnHist[key] = append(r.flow.fnHist[key], f)
			changed = true
		}
		if r.noteFnSite(key, f, s) {
			changed = true
		}
	}
	return changed
}

// order is where the write runs in the key's own body, note the write to cite; they differ for a callee's write applied at its call.
func (r *Result) noteFnSite(key fieldFnKey, f EntityID, s fnSiteSrc) bool {
	k := fieldSiteKey{Key: key, Fn: f}
	here := fieldPoisonSiteInfo{File: s.file, Node: s.node}
	note := here
	if s.hasFrom {
		if from, ok := r.fieldFnSites[fieldSiteKey{Key: s.from, Fn: f}]; ok {
			note = from
		}
	}
	if here.File != 0 && !slices.Contains(r.flow.sites[k], here) {
		if r.flow.sites == nil {
			r.flow.sites = map[fieldSiteKey][]fieldPoisonSiteInfo{}
		}
		r.flow.sites[k] = append(r.flow.sites[k], here)
	}
	changed := keepEarlier(r, &r.flow.order, k, here)
	if keepEarlier(r, &r.fieldFnSites, k, note) {
		changed = true
	}
	return changed
}

func keepEarlier[K comparable](r *Result, m *map[K]fieldPoisonSiteInfo, k K, site fieldPoisonSiteInfo) bool {
	if site.File == 0 {
		return false
	}
	if old, ok := (*m)[k]; ok && !r.siteEarlier(site, old) {
		return false
	}
	if *m == nil {
		*m = map[K]fieldPoisonSiteInfo{}
	}
	(*m)[k] = site
	return true
}

func (r *Result) siteEarlier(a, b fieldPoisonSiteInfo) bool {
	return a.File == b.File && r.Files[a.File].span(a.Node).Start < r.Files[b.File].span(b.Node).Start
}

func (r *Result) noteEdgeSite(key fieldFnKey, file FileID, node syntax.NodeID, tgt, param EntityID) {
	site, ok := r.fieldFnSites[fieldSiteKey{Key: key, Fn: tgt}]
	if !ok {
		return
	}
	if r.edgeFnSites == nil {
		r.edgeFnSites = map[edgeSiteKey]fieldPoisonSiteInfo{}
	}
	k := edgeSiteKey{File: file, Node: node, Target: tgt, Param: param}
	if _, seen := r.edgeFnSites[k]; !seen {
		r.edgeFnSites[k] = site
	}
}

func (r *Result) fieldWriteHop(e EffectEdge) (OpHop, bool) {
	site, ok := r.edgeFnSites[edgeSiteKey{File: e.File, Node: e.Node, Target: e.Target, Param: e.Param}]
	if !ok {
		return OpHop{}, false
	}
	name := "`" + r.Entities[e.Target].Name + "`"
	if r.Entities[e.Target].Kind == EntClosure {
		name = "this closure"
	}
	return OpHop{File: site.File, Node: site.Node, Text: name + " is stored in the field here"}, true
}

func (r *Result) noteEdgeSiteAt(file FileID, node syntax.NodeID, tgt, param EntityID, wfile FileID, wnode syntax.NodeID) {
	if r.edgeFnSites == nil {
		r.edgeFnSites = map[edgeSiteKey]fieldPoisonSiteInfo{}
	}
	k := edgeSiteKey{File: file, Node: node, Target: tgt, Param: param}
	if _, seen := r.edgeFnSites[k]; !seen {
		r.edgeFnSites[k] = fieldPoisonSiteInfo{File: wfile, Node: wnode}
	}
}
