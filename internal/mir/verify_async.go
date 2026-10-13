package mir

func verifyAsync(f *Func) *Error {
	seen := map[int]bool{}
	for bid, blk := range f.Blocks {
		bid := BlockID(bid)
		for i, in := range blk.Insts {
			switch in.Op {
			case OpAwait:
				switch {
				case !f.Async:
					return fail("async", bid, i, "await in a function that is not async")
				case in.Index < 1 || in.Index > f.Suspends:
					return fail("async", bid, i, "suspension point %d outside 1..%d", in.Index, f.Suspends)
				case seen[in.Index]:
					return fail("async", bid, i, "suspension point %d awaited twice", in.Index)
				}
				seen[in.Index] = true
			case OpFutureDone:
				if !f.Async {
					return fail("async", bid, i, "completion of a function that is not async")
				}
			}
		}
	}
	if !f.Async {
		return nil
	}
	if len(seen) != f.Suspends {
		return &Error{Rule: "async", Msg: itoa(f.Suspends) + " suspension points but " + itoa(len(seen)) + " awaits"}
	}
	if f.rcBase == 0 {
		return nil
	}
	return verifyFrame(f)
}

func verifyFrame(f *Func) *Error {
	if len(f.Slot) != len(f.Locals) {
		return &Error{Rule: "async", Msg: itoa(len(f.Slot)) + " frame slots for " + itoa(len(f.Locals)) + " locals"}
	}
	used := map[int]bool{}
	for l, s := range f.Slot {
		switch {
		case s < -1 || s >= f.Frame:
			return &Error{Rule: "async", Msg: "local " + itoa(l) + " in slot " + itoa(s) + " of a frame of " + itoa(f.Frame)}
		case s >= 0 && used[s]:
			return &Error{Rule: "async", Msg: "frame slot " + itoa(s) + " holds two locals"}
		}
		used[s] = true
	}
	if len(f.Cleanup) != f.Suspends+1 {
		return &Error{Rule: "async", Msg: itoa(len(f.Cleanup)) + " cleanup lists for " + itoa(f.Suspends) + " suspension points"}
	}
	check := func(what string, ls []LocalID) *Error {
		for _, l := range ls {
			if int(l) >= len(f.Locals) || f.Slot[l] < 0 {
				return &Error{Rule: "async", Msg: what + " releases local " + itoa(int(l)) + ", which has no frame slot"}
			}
		}
		return nil
	}
	for k, c := range f.Cleanup {
		if err := check("cleanup "+itoa(k), c); err != nil {
			return err
		}
	}
	return check("panic cleanup", f.PanicDrop)
}
