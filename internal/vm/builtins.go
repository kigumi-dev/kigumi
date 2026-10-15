package vm

// isTrivialScalar reports whether k is a value kind with no drop hook of
// its own: sharing it needs no ownership transfer, only a retain.
func isTrivialScalar(k kind) bool {
	switch k {
	case kInt, kFloat, kBool, kStr, kBytes, kChar, kUnit:
		return true
	}
	return false
}

// builtin is rt_builtin: the compiler-known runtime entry points.
func (m *Machine) builtin(name string, args []*obj) *obj {
	switch name {
	case "print", "eprint":
		line := append([]byte(m.display(args[0])), '\n')
		if name == "eprint" {
			m.writeErr(line)
		} else {
			m.writeOut(line)
		}
		return unitObj
	case "panic":
		if len(args) > 0 {
			m.abort(m.display(args[0]))
		}
		m.abort("explicit panic")
	case "host":
		if m.opts.Sandbox {
			m.abort("`host` is not available at compile time")
		}
		return mkOpaque("Host", nil)
	case "direct.eq":
		return mkBool(m.directEq(args[0], args[1]))
	case "direct.hash":
		return mkInt(int64(m.directHash(args[0])), 64)
	case "fixed.fill":
		n := deref(args[1]).i
		items := make([]*obj, n)
		for i := range items {
			items[i] = m.copy(args[0])
		}
		out := newObj(kArray)
		out.fields = items
		m.chargeMemory(out)
		return out
	case "alloc.push":
		// The VM allocates from Go; an `allocator` block only scopes.
		return mkOpaque("AllocatorScope", nil)
	case "iter.len":
		it := deref(args[0])
		switch it.k {
		case kArray:
			return mkInt(int64(len(it.fields)), 64)
		case kBytes, kStr:
			return mkInt(int64(len(it.s)), 64)
		case kRecord:
			if it.ent == m.r.MapEntity() {
				return mkInt(int64(len(it.fields[0].fields)), 64)
			}
		}
		lo, hi := deref(it.fields[0]), deref(it.fields[1])
		if is128(lo.nk) {
			return mkInt(rangeLen128(lo, hi, deref(it.fields[2]).b), 64)
		}
		if reversed := lo.nk&256 != 0 && lo.i > hi.i || lo.nk&256 == 0 && uint64(lo.i) > uint64(hi.i); reversed {
			return mkInt(0, 64)
		}
		n := int64(uint64(hi.i) - uint64(lo.i))
		if deref(it.fields[2]).b {
			n++
		}
		return mkInt(n, 64)
	case "iter.at", "iter.at_ref", "iter.at_move":
		it, i := deref(args[0]), deref(args[1]).i
		switch it.k {
		case kArray:
			switch name {
			case "iter.at_ref":
				// A borrowed element is never dropped by its own binding
				// (declare() excludes KRef locals), so it must not retain
				// either: nothing would release the extra count.
				return it.fields[i]
			case "iter.at_move":
				// The owned form of a non-Copy element moves it out:
				// the slot is tombstoned so the array's own eventual
				// drop never double-releases it. A generic loop body compiles
				// once per T, so this also runs for a Copy instantiation
				// (Array[T].fold, no `T: Copy` bound); a trivial scalar has
				// no drop to order, and some std loops re-read the array
				// mid-loop by index (Array.reverse), so it keeps the retain
				// instead.
				if isTrivialScalar(it.fields[i].k) {
					return retain(it.fields[i])
				}
				v := it.fields[i]
				it.fields[i] = unitObj
				return v
			}
			return retain(it.fields[i])
		case kBytes, kStr:
			return mkInt(int64(it.s[i]), 8)
		case kRecord:
			if it.ent == m.r.MapEntity() {
				tup, _ := m.r.Types.TupleEntity(2)
				keys, vals := it.fields[0], it.fields[1]
				if name == "iter.at_ref" {
					return mkRecord(tup, []*obj{keys.fields[i], vals.fields[i]})
				}
				if name == "iter.at_move" {
					k, v := keys.fields[i], vals.fields[i]
					if isTrivialScalar(k.k) {
						k = retain(k)
					} else {
						keys.fields[i] = unitObj
					}
					if isTrivialScalar(v.k) {
						v = retain(v)
					} else {
						vals.fields[i] = unitObj
					}
					return mkRecord(tup, []*obj{k, v})
				}
				return mkRecord(tup, []*obj{retain(keys.fields[i]), retain(vals.fields[i])})
			}
		}
		lo := deref(it.fields[0])
		if is128(lo.nk) {
			return mkWide(addW(widen(lo), wide{lo: uint64(i)}), lo.nk)
		}
		return mkInt(lo.i+i, lo.nk)
	}
	panic(&Unsupported{What: "builtin " + name})
}
