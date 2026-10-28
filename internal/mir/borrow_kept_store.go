package mir

import "kigumi/internal/sem"

func (c *borrowCheck) keptStoreFlow(e sem.EntityID, args []LocalID, s *keptSummary, t int) (storeFlow, bool) {
	info, params := c.p.R.Fn(e), c.declaredParams(e)
	if t >= len(params) {
		return storeFlow{}, false
	}
	fl, ok := c.keptTarget(info, params[t], args[t])
	if !ok {
		return storeFlow{}, false
	}
	fl.callee, fl.param, fl.args = e, t, args
	for k, a := range args {
		if k == t || k >= len(params) {
			continue
		}
		tie := tieNone
		if s.storesInto(t, k) {
			tie = c.keptTie(info, params[k])
		}
		switch {
		case tie != tieNone:
			fl.values = append(fl.values, flowArg{arg: a, tie: tie, param: k})
		case s.movesInto(t, k) && c.lentByCaller(info, params[k]):
			fl.values = append(fl.values, flowArg{arg: c.viewSource(a), contents: true, param: k})
		}
	}
	return fl, true
}
