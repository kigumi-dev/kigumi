package mir

import "kigumi/internal/sem"

// checkCapture rejects a holder stored into a record/variant/array, a closure/future
// capture, or a field/index/box write: none can outlive the borrow.
func (c *borrowCheck) checkCapture(in Inst) {
	switch in.Op {
	case OpRecord, OpClosure:
		// checkImmediateCaptures verifies a capBorrow closure's owners over its real span instead.
		if in.Op == OpClosure && c.p.R.Entity(in.Ent).Kind == sem.EntClosure && c.p.R.Closure(in.Ent).Immediate {
			return
		}
		// A record that itself became a holder routes the borrow through
		// its own `['a]`; it isn't escaping here.
		if in.Op == OpRecord {
			if _, ok := c.holders[in.Dst]; ok {
				c.checkHolderFields(in)
				return
			}
		}
		for _, a := range in.Args {
			if _, ok := c.holders[a]; ok {
				c.report(in.Node, a, "borrow-escape", "a borrow cannot be stored")
			}
		}
	case OpNewCell:
		if _, ok := c.holders[in.Args[0]]; ok {
			c.report(in.Node, in.Args[0], "borrow-escape", "a borrow cannot be stored")
		}
	case OpVariant:
		// A variant with its own ['a] routes a payload through it, same as OpRecord.
		if _, ok := c.holders[in.Dst]; ok {
			c.checkHolderPayloads(in)
			return
		}
		c.checkPayloadBorrows(in)
	case OpArray:
		c.checkElementBorrows(in)
	case OpFutureNew:
		// Same holder-record exception, for a future's own captures.
		if _, ok := c.holders[in.Dst]; ok {
			return
		}
		for _, a := range in.Args[1:] {
			if _, ok := c.holders[a]; ok {
				c.report(in.Node, a, "borrow-escape", "a borrow cannot be captured by a future")
			}
		}
	case OpCall, OpCallValue:
		c.checkOwningArgs(in)
	case OpSetField, OpSetIndex:
		if _, ok := c.holders[in.Args[len(in.Args)-1]]; ok {
			c.report(in.Node, in.Args[len(in.Args)-1], "borrow-escape", "a borrow cannot be stored")
		}
	}
}
