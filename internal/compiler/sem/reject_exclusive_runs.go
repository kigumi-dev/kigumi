package sem

import "kigumi/internal/compiler/syntax"

// argsRunCode reports whether evaluating the arguments may run user code: a call, an await, or the drop of a value whose
// type may have a destructor. Building a closure runs none of its body.
func (c *checker) argsRunCode(args []syntax.NodeID) bool {
	moved := map[syntax.NodeID]bool{}
	for _, a := range args {
		if c.t.Kind(a) == syntax.NamedArg {
			a = c.t.View(a).AsNamedArg().Value().ID()
		}
		c.markArgValue(a, moved)
	}
	var runs func(n syntax.NodeID) bool
	runs = func(n syntax.NodeID) bool {
		switch c.t.Kind(n) {
		case syntax.Lambda:
			return false
		case syntax.AwaitExpr:
			return true
		}
		if info, ok := c.info.Calls[n]; ok && !callRunsNoCode[info.Kind] {
			return true
		}
		if t := c.info.Types[n]; t != 0 && !moved[n] && c.r.dropMayRunCode(c.vars.resolve(t), map[TypeID]bool{}) {
			return true
		}
		for ch := range c.t.View(n).Children() {
			if runs(ch.ID()) {
				return true
			}
		}
		return false
	}
	for _, a := range args {
		if runs(a) {
			return true
		}
	}
	return false
}

// markArgValue marks the nodes whose value becomes the argument itself: the callee gets it, so evaluating the argument
// does not drop it.
func (c *checker) markArgValue(n syntax.NodeID, moved map[syntax.NodeID]bool) {
	moved[n] = true
	v := c.t.View(n)
	switch v.Kind() {
	case syntax.Paren:
		c.markArgValue(v.AsParen().Inner().ID(), moved)
	case syntax.Block:
		if tail := v.AsBlock().Tail(); tail.Valid() {
			c.markArgValue(tail.ID(), moved)
		}
	case syntax.RecordLit:
		for f := range v.AsRecordLit().Fields().Views() {
			if f.Kind() == syntax.FieldInit {
				c.markArgValue(f.AsFieldInit().Value().ID(), moved)
			}
		}
	case syntax.TupleLit:
		for e := range v.Children() {
			c.markArgValue(e.ID(), moved)
		}
	}
}

// callRunsNoCode lists the call kinds that build or read a value without running a function body.
var callRunsNoCode = map[CallKind]bool{
	CallVariant: true, CallBuiltinOp: true, CallIndex: true, CallMethodValue: true, CallRecord: true, CallDefinedWrap: true, CallFnItem: true,
}

// dropMayRunCode reports whether dropping an owned t may run a destructor: a resource, a value an unknown type may
// hide (a type parameter, an interface), or a part or closure capture that may.
func (r *Result) dropMayRunCode(t TypeID, visited map[TypeID]bool) bool {
	if t == 0 || visited[t] {
		return false
	}
	visited[t] = true
	n := r.Types.Node(t)
	switch n.Kind {
	case KParam, KVar, KIface:
		return true
	case KClosure:
		if n.Ent == 0 {
			return true
		}
		for _, cap := range r.closure(n.Ent).Captures {
			if r.dropMayRunCode(r.Entities[cap.Local].Type, visited) {
				return true
			}
		}
		return false
	case KNamed:
		if decl := r.typeDecl(n.Ent); decl != nil {
			return decl.Form == FormResource || r.partsMayRunCode(decl, n.Args, visited)
		}
	}
	for _, a := range n.Args {
		if r.dropMayRunCode(a, visited) {
			return true
		}
	}
	return false
}

func (r *Result) partsMayRunCode(decl *TypeDeclInfo, args []TypeID, visited map[TypeID]bool) bool {
	subst := instSubst(decl.Params, args)
	parts := []TypeID{decl.Underlying}
	for _, fld := range decl.Fields {
		parts = append(parts, r.field(fld).Type)
	}
	for _, v := range decl.Variants {
		parts = append(parts, r.variant(v).Payload...)
	}
	parts = append(parts, args...)
	for _, p := range parts {
		if p != 0 && r.dropMayRunCode(r.Types.Subst(p, subst), visited) {
			return true
		}
	}
	return false
}
