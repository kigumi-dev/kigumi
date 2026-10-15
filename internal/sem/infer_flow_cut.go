package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

type flowState struct {
	// fnHist survives a later poison of the key.
	fnHist map[fieldFnKey][]EntityID
	// bindHist keeps the param binds a poison retracted.
	bindHist   map[EntityID][]histBind
	scans      map[scanKey]*flowScan
	unresolved EntityID
	paramCopy  map[EntityID][]EntityID
	// fwdCalled is kept apart from CalledFieldParams, which the effect checks read.
	fwdCalled      map[EntityID][]FieldParamKey
	resultParams   map[EntityID][]resultParam
	strongWrites   map[EntityID][]strongWrite
	copyDeferred   map[EntityID]bool
	directDeferred map[EntityID]bool
	bindSrc        map[edgeSiteKey]fieldFnKey
	skipSets       [][]EntityID
	unionSeen      map[fnUnion]bool
	order          map[fieldSiteKey]fieldPoisonSiteInfo
	poisonAt       map[fieldFnKey]fieldPoisonSiteInfo
	// sites lists every statement that wrote the fn into the key, not only the earliest.
	sites    map[fieldSiteKey][]fieldPoisonSiteInfo
	kills    map[EntityID][]killSite
	soleElem map[EntityID]token.Span
}

type histBind struct {
	owner EntityID
	edge  EffectEdge
}

type scanKey struct {
	file FileID
	node syntax.NodeID
}

type flowScan struct {
	loops   []token.Span
	lambdas []token.Span
	// calls maps a lambda bound by `let` to the spans where the binding is called or passed on; absent when the binding is used any other way.
	calls map[token.Span][]token.Span
}

// copyCut is where a copy of a local was made.
type copyCut struct {
	file FileID
	node syntax.NodeID
	root EntityID
}

func within(outer, inner token.Span) bool {
	return outer.Start <= inner.Start && inner.End <= outer.End
}

func (r *Result) flowScanOf(file FileID, fnNode syntax.NodeID) *flowScan {
	k := scanKey{file, fnNode}
	if s, ok := r.flow.scans[k]; ok {
		return s
	}
	t := r.Files[file].Tree
	s := &flowScan{}
	lc := lambdaCalls{r: r, file: file, tree: t, scan: s, bound: map[EntityID]token.Span{}, direct: map[syntax.NodeID]bool{}, bad: map[token.Span]bool{}}
	var walk func(n syntax.NodeID)
	walk = func(n syntax.NodeID) {
		switch t.Kind(n) {
		case syntax.ForExpr:
			s.loops = append(s.loops, t.Span(n))
		case syntax.Lambda:
			s.lambdas = append(s.lambdas, t.Span(n))
		}
		lc.visit(n)
		for ch := range t.View(n).Children() {
			walk(ch.ID())
		}
	}
	for ch := range t.View(fnNode).Children() {
		walk(ch.ID())
	}
	lc.finish()
	if r.flow.scans == nil {
		r.flow.scans = map[scanKey]*flowScan{}
	}
	r.flow.scans[k] = s
	return s
}

// writeMayPrecede is true unless the write is certainly after the copy: later in the same body, outside any loop that also holds the copy.
func (r *Result) writeMayPrecede(site fieldPoisonSiteInfo, cut copyCut) bool {
	if site.File == 0 || cut.root == 0 || site.File != cut.file {
		return true
	}
	root := &r.Entities[cut.root]
	if (root.Kind != EntLocal && root.Kind != EntParam) || root.Parent == 0 || root.File != cut.file {
		return true
	}
	fn := &r.Entities[root.Parent]
	if fn.File != cut.file || fn.Node == 0 {
		return true
	}
	file := r.Files[cut.file]
	fnSpan, cutSpan, siteSpan := file.span(fn.Node), file.span(cut.node), file.span(site.Node)
	if !within(fnSpan, cutSpan) || !within(fnSpan, siteSpan) {
		return true
	}
	decl := file.span(root.Node).Start
	scan := r.flowScanOf(cut.file, fn.Node)
	holdsDecl := func(sp token.Span) bool { return sp.Start <= decl && decl < sp.End }
	for _, lam := range scan.lambdas {
		if within(lam, cutSpan) && !holdsDecl(lam) {
			return true
		}
	}
	for _, at := range scan.runSpans(siteSpan, cutSpan) {
		if at.Start < cutSpan.Start {
			return true
		}
		for _, lp := range scan.loops {
			if within(lp, cutSpan) && within(lp, at) && !holdsDecl(lp) {
				return true
			}
		}
	}
	return false
}

func (r *Result) visibleFns(key fieldFnKey, cut copyCut) []EntityID {
	var out []EntityID
	for _, f := range r.flow.fnHist[key] {
		site, ok := r.flow.order[fieldSiteKey{Key: key, Fn: f}]
		if (!ok || r.writeMayPrecede(site, cut)) && !r.fnKilledAt(key, f, cut.file, cut.node) {
			out = append(out, f)
		}
	}
	return out
}

// A poison with no recorded site is taken to precede every cut.
func (r *Result) poisonVisible(key fieldFnKey, cut copyCut) bool {
	if !r.fieldWitnessPoisoned[key] {
		return false
	}
	site, ok := r.flow.poisonAt[key]
	return !ok || !r.fnTypedFieldKey(key) || r.writeMayPrecede(site, cut)
}
