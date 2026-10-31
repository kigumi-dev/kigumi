package layout

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// scalarSlotBytes holds any register scalar unboxed, an i64 included on a 32-bit target.
const scalarSlotBytes = 8

// ObjSlotBytes is RT_OBJ_SLOT: the storage one runtime object is built in.
const ObjSlotBytes = 64

// Slot offsets stay in words so the runtime's nslots keeps its meaning.
func LayoutFrame(r *sem.Result, f *midir.Func) {
	word := wordBytes(r)
	next := int64(0)
	for i := range f.Locals {
		if f.Slot[i] < 0 {
			continue
		}
		size, align := slotLayout(r, f, midir.LocalID(i))
		off := roundUp(next, max(align, word))
		f.Slot[i] = int(off / word)
		next = off + roundUp(size, word)
	}
	f.ViewSlot = nil
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != midir.OpLendBox {
				continue
			}
			if f.ViewSlot == nil {
				f.ViewSlot = map[midir.LocalID]int{}
			}
			off := roundUp(next, max(scalarSlotBytes, word))
			f.ViewSlot[in.Dst] = int(off / word)
			next = off + ObjSlotBytes
		}
	}
	f.Frame = int(next / word)
}

func slotLayout(r *sem.Result, f *midir.Func, l midir.LocalID) (int64, int64) {
	t := f.Locals[l].Type
	switch {
	case f.IsDirect(l):
		return AggLayout(r, t)
	case r.RegScalar(t):
		return scalarSlotBytes, scalarSlotBytes
	}
	w := wordBytes(r)
	return w, w
}

func SlotBytes(r *sem.Result, f *midir.Func, l midir.LocalID) int64 {
	size, _ := slotLayout(r, f, l)
	return roundUp(size, wordBytes(r))
}

// The header is kgframe's desc, awaited, state, nargs and args, with slots[] 8-byte aligned.
func AsyncFrameBytes(r *sem.Result, f *midir.Func) uint64 {
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
