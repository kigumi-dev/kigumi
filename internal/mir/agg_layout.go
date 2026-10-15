package mir

import "kigumi/internal/sem"

const adtTagBytes = 4

// AggLayout must agree with llgen's layout, which checks it for every aggregate it emits.
func AggLayout(r *sem.Result, t sem.TypeID) (size, align int64) {
	if r.DirectAdt(t) {
		return adtLayout(r, t)
	}
	if elem, n, ok := r.FixedElem(t); ok {
		s, a := leafLayout(r, elem, true)
		return roundUp(s, a) * n, a
	}
	size, align = 0, 1
	for _, ft := range r.AggFields(t) {
		s, a := leafLayout(r, ft, false)
		size = roundUp(size, a) + s
		align = max(align, a)
	}
	return roundUp(size, align), align
}

// A narrow integer keeps its own width in an array but widens to a register in a field.
func leafLayout(r *sem.Result, t sem.TypeID, element bool) (int64, int64) {
	tt := r.Types
	switch {
	case t == sem.TyUnit:
		return 0, 1
	case t == sem.TyBool:
		return 1, 1
	case t == sem.TyChar:
		return 4, 4
	case r.DirectAgg(t):
		return AggLayout(r, t)
	case sem.WideScalar(t):
		return 16, 8
	case element && tt.IsInteger(t) && tt.Width(t) < 64:
		w := int64(tt.Width(t)) / 8
		return w, w
	}
	return 8, 8
}

func adtLayout(r *sem.Result, t sem.TypeID) (int64, int64) {
	var sizes, aligns []int64
	for _, v := range r.AggVariants(t) {
		for j, pt := range r.AggPayload(t, v) {
			if j >= len(sizes) {
				sizes, aligns = append(sizes, 0), append(aligns, 1)
			}
			s, a := leafLayout(r, pt, false)
			sizes[j], aligns[j] = max(sizes[j], s), max(aligns[j], a)
		}
	}
	cur, pay := int64(0), int64(1)
	for j := range sizes {
		cur = roundUp(cur, aligns[j]) + sizes[j]
		pay = max(pay, aligns[j])
	}
	words := roundUp(cur, pay) / pay
	align := max(adtTagBytes, pay)
	return roundUp(roundUp(adtTagBytes, pay)+words*pay, align), align
}

func roundUp(n, a int64) int64 { return (n + a - 1) / a * a }
