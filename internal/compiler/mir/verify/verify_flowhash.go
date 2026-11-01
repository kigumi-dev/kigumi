package verify

import midir "kigumi/internal/compiler/mir/ir"

func flowHash(f *midir.Func) uint64 {
	h := uint64(14695981039346656037)
	mix := func(x uint64) {
		h ^= x
		h *= 1099511628211
	}
	flag := func(b bool) uint64 {
		if b {
			return 1
		}
		return 0
	}
	mix(uint64(f.RCBase)<<1 | flag(f.Async))
	mix(uint64(f.Ret))
	for _, prm := range f.Params {
		mix(uint64(prm))
	}
	for _, l := range f.Locals {
		mix(uint64(l.Type)<<8 | uint64(l.Ent)<<40 | flag(l.Cell) | flag(l.Borrowed)<<1 | flag(l.Cross)<<2)
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			mix(uint64(in.Op) | uint64(in.Dst)<<8 | uint64(in.Index)<<40)
			mix(uint64(in.Ent) | uint64(in.Type)<<32)
			mix(flag(in.Share) | flag(in.Copy)<<1 | flag(in.Async)<<2 | flag(in.Fixed)<<3 | flag(in.Scoped)<<4 | flag(in.EndScope)<<5 | flag(in.Lit.Bool)<<6 | uint64(in.Lit.Kind)<<7)
			for i := 0; i < len(in.Str); i++ {
				mix(uint64(in.Str[i]))
			}
			for _, a := range in.Args {
				mix(uint64(a))
			}
			mix(uint64(len(in.Args)))
		}
		mix(uint64(blk.Term.Op) | uint64(len(blk.Insts))<<8)
		for _, a := range blk.Term.Args {
			mix(uint64(a))
		}
		for _, t := range blk.Term.Targets {
			mix(uint64(t))
		}
	}
	return h
}
