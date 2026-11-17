package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// threadCheck is a Send or Sync bound over a closure or a function value, judged by checkThreadBounds once captures,
// context bindings and a value's calls are final; site is the fn or lambda whose body makes the call.
type threadCheck struct {
	node     syntax.NodeID
	arg      TypeID
	con      Constraint
	param    EntityID
	flow     *fnFlow
	held     *valueInstData
	site     EntityID
	producer EntityID
}

// pendingThread is a threadCheck with what its diagnostic needs from the checker that made it.
type pendingThread struct {
	threadCheck
	file  FileID
	label any
	text  string
}

// deferThread keeps a Send or Sync bound over a closure, or over a function type some function item may meet, for
// checkThreadBounds.
func (c *checker) deferThread(cc constraintCheck, arg TypeID, con Constraint) bool {
	if con.Kind != CSend && con.Kind != CSync {
		return false
	}
	tc := threadCheck{node: cc.node, arg: arg, con: con, param: cc.param, site: cc.site, producer: c.resultProducer(cc)}
	isFn := c.r.Types.Kind(arg) == KFn
	switch {
	case !isFn && !c.r.Types.mentionsClosure(arg):
		return false
	case cc.value && cc.held != nil:
		tc.held = cc.held
	case isFn && c.callFlows[cc.node][cc.param] != nil:
		tc.flow = c.callFlows[cc.node][cc.param]
	case isFn:
		return false
	}
	c.threadChecks = append(c.threadChecks, tc)
	return true
}

func (c *checker) flushThreadChecks() {
	for _, tc := range c.threadChecks {
		c.r.thread.checks = append(c.r.thread.checks, pendingThread{tc, c.f, c.typeArgLabel(tc.arg), c.constraintText(tc.con)})
	}
	c.threadChecks = nil
}

func (tt *TypeTable) mentionsClosure(t TypeID) bool {
	if t == 0 {
		return false
	}
	n := tt.Node(t)
	if n.Kind == KClosure {
		return true
	}
	if n.Elem != 0 && tt.mentionsClosure(n.Elem) {
		return true
	}
	for _, a := range n.Args {
		if tt.mentionsClosure(a) {
			return true
		}
	}
	return false
}

func (c *checker) resultProducer(cc constraintCheck) EntityID {
	tt := c.r.Types
	fn := c.r.Entities[cc.param].Parent
	if c.r.Entities[fn].Kind != EntFn {
		return 0
	}
	info := c.r.Fn(fn)
	params := slices.Clone(tt.Node(info.Sig).Args)
	if info.SelfParam != 0 {
		params = append(params, c.r.Entities[info.SelfParam].Type)
	}
	for _, p := range params {
		if slices.Contains(tt.paramsIn(p, nil), cc.param) {
			return 0
		}
	}
	for _, f := range slices.Concat(info.TypeParams, info.RecvParams) {
		for _, con := range c.r.typeParam(f).Constraints {
			ret := tt.Node(con.Type).Elem
			if con.Kind != CFnOnce || tt.Kind(ret) != KParam || tt.Node(ret).Ent != cc.param {
				continue
			}
			if got := c.vars.resolve(cc.subst[f]); tt.Kind(got) == KClosure {
				return tt.Node(got).Ent
			}
		}
	}
	return 0
}
