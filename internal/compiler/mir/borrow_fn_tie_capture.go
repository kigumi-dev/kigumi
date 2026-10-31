package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// returnedCaptures: a closure returning a borrow keeps its non-owned captures that can build the result borrowed.
func (c *borrowCheck) returnedCaptures(def *midir.Inst) *borrowInfo {
	info := c.p.R.Closure(def.Ent)
	if !c.resultMayBorrow(info.Ret) || len(info.Captures) != len(def.Args) {
		return nil
	}
	res := info.Ret
	for i := 0; i < 8 && c.fnResult(res) != 0; i++ {
		res = c.fnResult(res)
	}
	mut, viaFn := c.holdsMutRef(res), !c.mayBorrowResult(res)
	if viaFn {
		mut = c.carriesMaker(res, map[sem.TypeID]bool{}, func(r sem.TypeID) bool { return c.mayBorrowResult(r) && c.holdsMutRef(r) })
	}
	out := &borrowInfo{hasOwner: true, node: def.Node}
	for i, cp := range info.Captures {
		if cp.Mode == sem.CapCopy || cp.Mode == sem.CapMove || cp.Mode == sem.CapRetain || !viaFn && !c.reaches(c.p.R.Entity(cp.Local).Type, false, info.Ret) {
			continue
		}
		if root, path, exact, ok := c.projectionPlace(def.Args[i]); ok {
			out.owners = append(out.owners, ownerRef{owner: root, mut: mut && cp.Mode != sem.CapRead, path: path, exact: exact, node: def.Node})
		}
	}
	if len(out.owners) == 0 {
		return nil
	}
	return out
}

// resultMayBorrow also looks into a payload, field or element holding a fn value whose result may borrow.
func (c *borrowCheck) resultMayBorrow(t sem.TypeID) bool {
	for range 8 {
		if c.mayBorrowResult(t) {
			return true
		}
		r := c.fnResult(t)
		if r == 0 {
			return c.mayHoldFn(t) && c.carriesBorrowMaker(t, map[sem.TypeID]bool{})
		}
		t = r
	}
	return true
}

// borrowRoots adds the locals a closure captures without owning to borrowRoot's owners.
func (c *borrowCheck) borrowRoots(in midir.Inst) []midir.LocalID {
	if root, ok := c.borrowRoot(in); ok {
		return []midir.LocalID{root}
	}
	if in.Op != midir.OpClosure || c.p.R.Entity(in.Ent).Kind != sem.EntClosure {
		return nil
	}
	info := c.p.R.Closure(in.Ent)
	if len(info.Captures) != len(in.Args) {
		return nil
	}
	var out []midir.LocalID
	for i, cp := range info.Captures {
		if cp.Mode == sem.CapCopy || cp.Mode == sem.CapMove || cp.Mode == sem.CapRetain {
			continue
		}
		if root, _, _, ok := c.projectionPlace(in.Args[i]); ok {
			out = append(out, root)
		}
	}
	return out
}
