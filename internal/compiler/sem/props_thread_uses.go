package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

type closureUse struct {
	node  syntax.NodeID
	hides bool
}

type callRef struct {
	file FileID
	node syntax.NodeID
}

func (r *Result) closureUses(cl EntityID) []closureUse {
	st := &r.thread
	f := r.Entities[cl].File
	if st.uses == nil {
		st.uses = map[FileID]map[EntityID][]closureUse{}
	}
	idx, ok := st.uses[f]
	if !ok {
		idx = r.indexClosureUses(f)
		st.uses[f] = idx
	}
	return idx[cl]
}

func (r *Result) indexClosureUses(f FileID) map[EntityID][]closureUse {
	fi := &r.Files[f]
	out := map[EntityID][]closureUse{}
	for n, ci := range fi.Calls {
		// A synthesized default thunk names the holder's type in its type arguments but takes no value of it.
		if ci.Kind == CallVariant || ci.Kind == CallRecord || ci.Kind == CallDefinedWrap || fi.Defaults[n] != 0 {
			continue
		}
		var held []EntityID
		for _, t := range r.callOperandTypes(fi, n, ci) {
			for _, cl := range r.keptClosures(t) {
				if !slices.Contains(held, cl) {
					held = append(held, cl)
				}
			}
		}
		hides := r.hidesValues(fi.Types[n])
		for _, cl := range held {
			out[cl] = append(out[cl], closureUse{n, hides})
		}
	}
	for n, co := range fi.Coerce {
		if len(co.Steps) == 0 {
			continue
		}
		tgt := co.Steps[len(co.Steps)-1].To
		for _, cl := range r.keptClosures(co.From) {
			// A coercion that drops the closure from the value's type, as an erasure to an interface does, hides it.
			if !slices.Contains(r.keptClosures(tgt), cl) || r.hidesValues(tgt) {
				out[cl] = append(out[cl], closureUse{n, true})
			}
		}
	}
	return out
}

func (r *Result) callOperandTypes(fi *FileInfo, n syntax.NodeID, ci CallInfo) []TypeID {
	t := fi.Tree
	out := slices.Clone(ci.Inst)
	var callee syntax.NodeID
	switch t.Kind(n) {
	case syntax.CallExpr:
		callee = t.View(n).AsCall().Callee().ID()
	case syntax.WsCallExpr:
		callee = t.View(n).AsWsCall().Callee().ID()
	case syntax.Binary:
		callee = t.View(n).AsBinary().Right().ID()
	}
	if callee != 0 {
		callee = t.Unparen(callee)
		out = append(out, fi.Types[callee])
		if base := t.View(callee).Base(); base.Valid() {
			out = append(out, fi.Types[base.ID()])
		}
	}
	if ci.Plan != nil {
		for _, a := range ci.Plan.Eval {
			out = append(out, fi.Types[a])
		}
	}
	return out
}

// Any other call taking h may have made a reader of h's local that is still alive.
func (r *Result) takenOnlyAt(h EntityID, at callRef) bool {
	for _, u := range r.closureUses(h) {
		if at.file != r.Entities[h].File || u.node != at.node {
			return false
		}
	}
	return true
}
