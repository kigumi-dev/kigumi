package borrow

import (
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) checkPayloadBorrows(in midir.Inst) {
	vi := c.p.R.Variant(in.Ent)
	owner := c.p.R.Entity(in.Ent).Parent
	for i, a := range in.Args {
		if _, ok := c.holders[a]; !ok {
			continue
		}
		c.reportCode(in.Node, a, "borrow-in-payload", payloadName(vi, i), c.p.R.Entity(owner).Name)
	}
}

// checkElementBorrows is checkPayloadBorrows for OpArray: an element has no
// declared name, so it names the array's own type instead.
func (c *borrowCheck) checkElementBorrows(in midir.Inst) {
	t := c.p.R.TypeString(c.f.Locals[in.Dst].Type)
	for i, a := range in.Args {
		if _, ok := c.holders[a]; !ok {
			continue
		}
		c.reportCode(in.Node, a, "borrow-in-element", fmt.Sprintf("element %d", i), t)
	}
}

// checkHolderPayloads is checkHolderFields for a variant: only a payload
// declared as a borrow, or routing one through its own ['a], is skipped.
func (c *borrowCheck) checkHolderPayloads(in midir.Inst) {
	vi := c.p.R.Variant(in.Ent)
	if len(vi.Payload) != len(in.Args) {
		return
	}
	owner := c.p.R.Entity(in.Ent).Parent
	for i, a := range in.Args {
		if !c.unownedRef(a) || c.declaredAsBorrow(vi.Payload[i]) {
			continue
		}
		c.reportCode(in.Node, a, "borrow-owned-payload", payloadName(vi, i), c.p.R.Entity(owner).Name)
	}
}

func payloadName(vi *sem.VariantInfo, i int) string {
	if i < len(vi.Names) && vi.Names[i] != "" {
		return "`" + vi.Names[i] + "`"
	}
	return fmt.Sprintf("payload %d", i)
}
