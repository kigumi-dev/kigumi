package vm

import "kigumi/internal/compiler/mir"

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
	case "atomic.load", "atomic.store", "atomic.exchange", "atomic.compareExchange", "atomic.add", "atomic.sub", "atomic.and", "atomic.or", "atomic.xor":
		return m.atomicStatic(m.site.Ent, name, args)
	case "volatile.read":
		return m.volatileReadAt(deref(args[0]).i)
	case "volatile.write":
		m.volatileWriteAt(deref(args[0]).i, deref(args[1]))
		return unitObj
	case "task.close":
		m.abort("`task.close` outside an async body")
	case "print", "eprint":
		line := append([]byte(m.display(args[0])), '\n')
		if name == "eprint" {
			m.writeErr(line)
			if m.opts.FaultArmOnEprint && m.sh.faultArmed.CompareAndSwap(false, true) {
				m.sh.faultN.Store(0)
			}
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
	case "cmp.unordered":
		isNaN := func(v *obj) bool { v = deref(v); return v.k == kFloat && v.fl() != v.fl() }
		return mkBool(mir.FloatWitnessKey(deref(args[0]).cstd()) && (isNaN(args[1]) || isNaN(args[2])))
	case "cmp.guard":
		return mkBool(deref(args[0]).b && !deref(args[1]).b)
	case "fixed.fill":
		return m.fillFixed(name, args[0], deref(args[1]).i)
	case "alloc.push":
		// One AllocGuard marks this dynamic extent: it pushes the arena on
		// creation and pops it when its final release ends the block.
		scope := deref(args[0])
		handle, _ := scope.opaqueData().(*obj)
		if handle == nil {
			m.abort("allocator scope has no handle")
		}
		st, _ := handle.opaqueData().(*arenaState)
		if st == nil {
			m.abort("allocator scope has no arena")
		}
		if st.closed {
			m.abort("arena is closed")
		}
		if len(m.allocStack) >= 64 {
			m.abort("allocator blocks nested too deeply")
		}
		m.allocStack = append(m.allocStack, st)
		return mkOpaque("AllocGuard", retain(scope))
	case "iter.len":
		it := derefRaw(args[0])
		switch it.k {
		case kRawArray:
			return mkInt(int64(len(it.rawArr().raw)), 64)
		case kArray:
			return mkInt(int64(len(it.fields)), 64)
		case kBytes, kStr:
			return mkInt(int64(len(it.str())), 64)
		case kRecord:
			if it.ent == m.r.MapEntity() {
				return mkInt(int64(len(it.fields[0].fields)), 64)
			}
		}
		lo, hi := deref(it.fields[0]), deref(it.fields[1])
		if is128(lo.nkind()) {
			return mkInt(rangeLen128(lo, hi, deref(it.fields[2]).b), 64)
		}
		if reversed := lo.nkind()&256 != 0 && lo.i > hi.i || lo.nkind()&256 == 0 && uint64(lo.i) > uint64(hi.i); reversed {
			return mkInt(0, 64)
		}
		n := int64(uint64(hi.i) - uint64(lo.i))
		if deref(it.fields[2]).b {
			n++
		}
		return mkInt(n, 64)
	case "iter.at", "iter.at_ref", "iter.at_move":
		it, i := derefRaw(args[0]), deref(args[1]).i
		switch it.k {
		case kRawArray:
			if name == "iter.at_ref" {
				return it.rawArr().borrowed(i)
			}
			return it.rawArr().at(i)
		case kArray:
			switch name {
			case "iter.at_ref":
				// Borrowed bindings never release their elements, so retaining would leak.
				return it.fields[i]
			case "iter.at_move":
				// Scalars stay readable by later loop iterations; other elements leave
				// a tombstone so dropping the array cannot release a moved value twice.
				if isTrivialScalar(it.fields[i].k) {
					return retain(it.fields[i])
				}
				v := it.fields[i]
				it.fields[i] = unitObj
				return v
			}
			return retain(it.fields[i])
		case kBytes, kStr:
			return mkInt(int64(it.str()[i]), 8)
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
		if is128(lo.nkind()) {
			return mkWide(addW(widen(lo), wide{lo: uint64(i)}), lo.nkind())
		}
		return mkInt(lo.i+i, lo.nkind())
	}
	if name == "comptime" {
		panic(&Unsupported{What: "reading a `comptime` value before its block is evaluated (declare it above the one that reads it)"})
	}
	panic(&Unsupported{What: "builtin " + name})
}
