package mir

import "kigumi/internal/compiler/sem"

// maxAggregateDepth stops recursion into a pathological type graph; by-value
// recursive types are already rejected elsewhere, so this is a backstop.
const maxAggregateDepth = 8

// frameBytes is a conservative per-function stack estimate: its own
// locals' sizes summed, not a real backend-measured frame size.
func frameBytes(r *sem.Result, f *Func, word uint64) uint64 {
	locals := f.Locals
	if f.rcBase > 0 {
		locals = locals[:f.rcBase]
	}
	var total uint64
	for _, l := range locals {
		total += localBytes(r, l.Type, word, 0)
	}
	return total
}

// localBytes treats a carrier-repr value as one handle word, not the record it boxes.
func localBytes(r *sem.Result, t sem.TypeID, word uint64, depth int) uint64 {
	if t == 0 || depth > maxAggregateDepth {
		return word
	}
	if r.Repr(t) == sem.ReprCarrier {
		return word
	}
	tt := r.Types
	switch {
	case tt.IsNumeric(t):
		if w := uint64(tt.Width(t)) / 8; w > word {
			return w
		}
		return word
	case t == sem.TyBool, t == sem.TyChar, tt.Kind(t) == sem.KPtr, tt.IsCFn(t):
		return word
	}
	if r.DirectClosure(t) {
		return aggBytes(r, r.AggFields(t), word, depth+1)
	}
	if elem, n, ok := r.FixedElem(t); ok && r.DirectFixed(t) {
		return max(uint64(n)*localBytes(r, elem, word, depth+1), word)
	}
	n := tt.Node(t)
	if n.Kind != sem.KNamed || n.Ent == 0 {
		return word
	}
	info := r.TypeDecl(n.Ent)
	switch info.Form {
	case sem.FormDefined:
		if info.Underlying == 0 {
			return word
		}
		return localBytes(r, r.UnderlyingRepr(t), word, depth+1)
	case sem.FormRecord, sem.FormResource:
		return fieldsBytes(r, info.Fields, word, depth+1)
	case sem.FormAdt:
		return adtBytes(r, info.Variants, word, depth+1)
	}
	return word
}

func fieldsBytes(r *sem.Result, fields []sem.EntityID, word uint64, depth int) uint64 {
	var total uint64
	for _, fld := range fields {
		total += localBytes(r, r.Entity(fld).Type, word, depth)
	}
	if total == 0 {
		return word
	}
	return total
}

// staticWritableBytes sums every Kigumi-owned `static mut`'s size (localBytes'
// own estimate); an `extern` static's storage lives outside this program.
func staticWritableBytes(r *sem.Result, word uint64) uint64 {
	var total uint64
	for _, id := range r.StaticEntities() {
		owner, mut := r.StaticOwnerOf(id)
		if owner == sem.StaticExtern || !mut {
			continue
		}
		total += localBytes(r, r.Entity(id).Type, word, 0)
	}
	return total
}

func aggBytes(r *sem.Result, leaves []sem.TypeID, word uint64, depth int) uint64 {
	var total uint64
	for _, t := range leaves {
		total += localBytes(r, t, word, depth)
	}
	return max(total, word)
}

// adtBytes takes the largest variant's payload plus one tag word: a
// conservative stand-in for the native tagged-union layout.
func adtBytes(r *sem.Result, variants []sem.EntityID, word uint64, depth int) uint64 {
	var worst uint64
	for _, v := range variants {
		info := r.Variant(v)
		var total uint64
		for _, pt := range info.Payload {
			total += localBytes(r, pt, word, depth)
		}
		if total > worst {
			worst = total
		}
	}
	return worst + word
}
