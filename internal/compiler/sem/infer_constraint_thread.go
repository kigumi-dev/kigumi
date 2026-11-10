package sem

import "kigumi/internal/compiler/syntax"

// threadCheck is a Send or Sync bound over a closure or a function value, judged by checkThreadBounds once captures,
// context bindings and a value's calls are final; site is the fn or lambda whose body makes the call.
type threadCheck struct {
	node  syntax.NodeID
	arg   TypeID
	con   Constraint
	param EntityID
	flow  *fnFlow
	held  *valueInstData
	site  EntityID
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
	tc := threadCheck{node: cc.node, arg: arg, con: con, param: cc.param, site: cc.site}
	switch {
	case c.r.Types.Kind(arg) == KFn && cc.value && cc.held != nil:
		tc.held = cc.held
	case c.r.Types.Kind(arg) == KFn && c.callFlows[cc.node][cc.param] != nil:
		tc.flow = c.callFlows[cc.node][cc.param]
	case !c.r.Types.mentionsClosure(arg):
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
