package mir

import "kigumi/internal/sem"

// scalarSlotBytes holds any register scalar unboxed, an i64 included on a 32-bit target.
const scalarSlotBytes = 8

// ObjSlotBytes is RT_OBJ_SLOT: the storage one runtime object is built in.
const ObjSlotBytes = 64

// Slot offsets stay in words so the runtime's nslots keeps its meaning.
func (p *Program) layoutFrame(f *Func) {
	word := wordBytes(p.R)
	next := int64(0)
	for i := range f.Locals {
		if f.Slot[i] < 0 {
			continue
		}
		size, align := p.slotLayout(f, LocalID(i))
		off := roundUp(next, max(align, word))
		f.Slot[i] = int(off / word)
		next = off + roundUp(size, word)
	}
	f.ViewSlot = nil
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != OpLendBox {
				continue
			}
			if f.ViewSlot == nil {
				f.ViewSlot = map[LocalID]int{}
			}
			off := roundUp(next, max(scalarSlotBytes, word))
			f.ViewSlot[in.Dst] = int(off / word)
			next = off + ObjSlotBytes
		}
	}
	f.Frame = int(next / word)
}

func (p *Program) slotLayout(f *Func, l LocalID) (int64, int64) {
	t := f.Locals[l].Type
	switch {
	case f.IsDirect(l):
		return AggLayout(p.R, t)
	case p.R.RegScalar(t):
		return scalarSlotBytes, scalarSlotBytes
	}
	w := wordBytes(p.R)
	return w, w
}

func (p *Program) SlotBytes(f *Func, l LocalID) int64 {
	size, _ := p.slotLayout(f, l)
	return roundUp(size, wordBytes(p.R))
}

// The header is kgframe's desc, awaited, state, nargs and args, with slots[] 8-byte aligned.
func AsyncFrameBytes(r *sem.Result, f *Func) uint64 {
	word := wordBytes(r)
	header := roundUp(3*word+8, scalarSlotBytes)
	return uint64(header + (int64(f.Frame)+int64(len(f.Params)))*word)
}

func wordBytes(r *sem.Result) int64 {
	if w := int64(r.Types.PtrBits()) / 8; w > 0 {
		return w
	}
	return 8
}
