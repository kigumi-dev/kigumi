package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

type fnResult struct {
	fns     []EntityID
	params  []int
	unknown bool
}

type fnResultWrite struct {
	key    fieldFnKey
	callee EntityID
	args   []syntax.NodeID
	caller EntityID
	file   FileID
	node   syntax.NodeID
}

func (r *Result) fnReturnsFn(fn EntityID) bool {
	info := r.Fn(fn)
	if info.Body == 0 || info.Declared&EffAsync != 0 {
		return false
	}
	switch r.Types.Kind(r.Types.Node(info.Sig).Elem) {
	case KFn, KClosure:
		return true
	}
	return false
}

func (c *checker) recordFnResultLeaves(body syntax.NodeID) {
	if c.r.Types.Kind(c.retType) != KFn && c.r.Types.Kind(c.retType) != KClosure {
		return
	}
	res := &fnResult{}
	if c.r.fnResults == nil {
		c.r.fnResults = map[EntityID]*fnResult{}
	}
	c.r.fnResults[c.fn] = res
	params := c.r.Fn(c.fn).Params
	for _, leaf := range c.resultLeaves(body) {
		if fns := c.knownFnRefs(leaf); len(fns) > 0 {
			for _, f := range fns {
				if !slices.Contains(res.fns, f) {
					res.fns = append(res.fns, f)
				}
			}
			continue
		}
		if p := c.identEnt(leaf); p != 0 && c.r.Entities[p].Kind == EntParam && slices.Contains(params, p) {
			if i := slices.Index(params, p); !slices.Contains(res.params, i) {
				res.params = append(res.params, i)
			}
			continue
		}
		res.unknown = true
	}
}

func (c *checker) stageFnResultWrite(key fieldFnKey, init syntax.NodeID) bool {
	init = c.stripParen(init)
	if c.t.Kind(init) != syntax.CallExpr {
		return false
	}
	call, ok := c.info.Calls[init]
	if !ok || call.Callee == 0 || c.r.Entities[call.Callee].Kind != EntFn || !c.r.fnReturnsFn(call.Callee) {
		return false
	}
	var args []syntax.NodeID
	if call.Plan != nil {
		args = call.Plan.placed()
	}
	c.r.fnResultWrites = append(c.r.fnResultWrites, fnResultWrite{key: key, callee: call.Callee, args: args, caller: c.fn, file: c.f, node: init})
	return true
}

func (r *Result) applyFnResultWrites() bool {
	changed := false
	for _, w := range r.fnResultWrites {
		if r.fieldWitnessPoisoned[w.key] {
			continue
		}
		res := r.fnResults[w.callee]
		if res == nil || res.unknown {
			if r.poisonFieldKeyAt(w.key, w.file, w.node) {
				changed = true
			}
			continue
		}
		if r.joinKnownFieldFnAt(w.key, res.fns, fnSiteSrc{file: w.file, node: w.node}) {
			changed = true
		}
		for _, i := range res.params {
			if i < len(w.args) && r.applyFnFieldWriteExpr(w.file, w.key, w.args[i], w.caller, w.node) {
				changed = true
			}
		}
	}
	return changed
}
