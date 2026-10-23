package sem

import (
	"strconv"

	"kigumi/internal/diag"
	"kigumi/internal/syntax"
)

func (c *checker) requiredParams(declParams []EntityID, fixed int) int {
	return c.r.requiredParams(declParams, fixed)
}

func arityCount(required, total int) any {
	if required < total {
		return strconv.Itoa(required) + " to " + strconv.Itoa(total)
	}
	return total
}

// fillDefaults appends a DefaultArg per empty fixed slot in declaration order; ok is false after reporting a missing required argument.
func (c *checker) fillDefaults(n syntax.NodeID, name string, eval []syntax.NodeID, place []int, declParams []EntityID, fixed int, inst []TypeID) ([]syntax.NodeID, []int, bool) {
	for len(place) < fixed {
		place = append(place, -1)
	}
	for slot := 0; slot < fixed && slot < len(declParams); slot++ {
		if place[slot] >= 0 {
			continue
		}
		param := declParams[slot]
		thunk := c.r.Entities[param].Target
		if thunk == 0 {
			c.errAt(n, cArgMissing, name, c.r.Entities[param].Name)
			return eval, place, false
		}
		d := c.t.AddDefaultArg(c.t.View(n).Tok().Index())
		c.growTables()
		if c.info.Defaults == nil {
			c.info.Defaults = map[syntax.NodeID]EntityID{}
		}
		c.info.Defaults[d] = thunk
		c.instantiateThunk(d, thunk, inst)
		eval = append(eval, d)
		place[slot] = len(eval) - 1
	}
	return eval, place, true
}

// instantiateThunk gives a generic thunk the call's type arguments: the receiver's, then the callee's own.
func (c *checker) instantiateThunk(d syntax.NodeID, thunk EntityID, inst []TypeID) {
	gen := c.r.Fn(thunk).TypeParams
	if len(gen) == 0 {
		return
	}
	args := inst
	if c.deflt.recv != 0 {
		recvArgs := c.r.Types.Node(c.deflt.recv).Args
		if n := len(c.r.ownerParams(thunk)); n <= len(recvArgs) {
			args = append(append([]TypeID(nil), recvArgs[:n]...), inst...)
		}
	}
	if len(args) != len(gen) {
		return
	}
	c.info.Calls[d] = CallInfo{Kind: CallFn, Callee: thunk, Inst: args}
	if len(inst) > 0 {
		c.r.Instances[thunk] = append(c.r.Instances[thunk], Instance{Args: inst, File: c.f, Node: d, In: c.fn})
	}
	c.deferWitnesses(d, thunk, c.deflt.recv, inst)
	c.touched = append(c.touched, d)
}

func (c *checker) synthDefaultArg(n syntax.NodeID) TypeID {
	thunk := c.info.Defaults[n]
	c.edge(EffectEdge{Kind: EdgeCall, Target: thunk, Node: n})
	info := c.r.Fn(thunk)
	ret := c.r.Types.Node(info.Sig).Elem
	generic, isGeneric := c.info.Calls[n]
	if isGeneric {
		ret = c.r.Types.Subst(ret, c.r.InstanceSubst(thunk, generic.Inst))
	}
	if !info.DefaultTry {
		return ret
	}
	val, e, ok := c.r.Types.IsResult(ret)
	if !ok {
		return ret
	}
	c.deflt.note = c.defaultTryNote(thunk, info)
	out := c.tryPropagate(n, ret, val, e, false)
	c.deflt.note = nil
	if isGeneric {
		if try := c.info.Calls[n]; try.Kind == CallTry && len(try.Inst) == 1 {
			generic.Box = try.Inst[0]
		}
		c.info.Calls[n] = generic
	}
	return out
}

func (c *checker) defaultTryNote(thunk EntityID, info *FnInfo) *diag.Diagnostic {
	f := c.r.Entities[thunk].File
	span := c.r.Files[f].Tree.Span(info.Body)
	return &diag.Diagnostic{Severity: diag.Note, Loc: c.r.loc(f, span), Msg: "the default of `" + info.DefaultParam + "` uses `?`, so a call that omits it propagates the error to its caller"}
}

// defaultFrame is the receiver type and the `?` note of the call whose defaults are being checked.
type defaultFrame struct {
	recv TypeID
	note *diag.Diagnostic
}
