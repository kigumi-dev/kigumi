package vm

import (
	"strings"

	"kigumi/internal/hashkey"
)

// prelude implements the Option / Result helpers, Bytes.tryConcat and the Eq / Ord / Hash
// witnesses of the primitive types; nil for other keys.
func (m *Machine) prelude(key string, a []*obj, a0, a1 *obj) *obj {
	switch key {
	case "prelude.debug":
		return mkStr([]byte(m.display(a0)))
	case "prelude.Bool.branch", "prelude.Option.branch", "prelude.Result.branch":
		branch := m.lookup("std/prelude", "Branch")
		hit, miss := m.variantOf(branch, "Hit"), m.variantOf(branch, "Miss")
		switch a0.k {
		case kBool:
			if a0.b {
				return mkVariant(hit, []*obj{trueObj})
			}
			return mkVariant(miss, []*obj{unitObj})
		}
		name := m.r.Entity(a0.ent).Name
		if name == "Some" || name == "Ok" {
			return mkVariant(hit, []*obj{retain(a0.fields[0])})
		}
		if len(a0.fields) > 0 {
			return mkVariant(miss, []*obj{retain(a0.fields[0])})
		}
		return mkVariant(miss, []*obj{unitObj})
	case "prelude.Bytes.tryConcat":
		m.reserveMemory(uint64(len(a0.str())) + uint64(len(a1.str())))
		if uint64(len(a0.str()))+uint64(len(a1.str())) >= maxVMAlloc || m.planRefused(key) {
			return m.allocError()
		}
		out := make([]byte, 0, len(a0.str())+len(a1.str()))
		out = append(out, a0.str()...)
		out = append(out, a1.str()...)
		return m.okPlanned(key, mkBytes(out))
	case "prelude.Option.take":
		if m.r.Entity(a0.ent).Name != "Some" {
			return m.none()
		}
		x := retain(a0.fields[0])
		m.cellSet(a[0], m.none())
		return m.some(x)
	}
	if strings.HasSuffix(key, ".equals") {
		if a0.k == kFloat {
			return mkBool(hashkey.TotalEqualFloat64(a0.fl(), a1.fl()))
		}
		return mkBool(m.equal(a0, a1))
	}
	if strings.HasSuffix(key, ".compareTo") {
		ord := m.lookup("std/prelude", "Ordering")
		c := compare(a0, a1)
		if a0.k == kFloat {
			c = hashkey.TotalCompareFloat64(a0.fl(), a1.fl())
		}
		name := "Equal"
		if c < 0 {
			name = "Less"
		} else if c > 0 {
			name = "Greater"
		}
		return mkVariant(m.variantOf(ord, name), nil)
	}
	if strings.HasSuffix(key, ".hash") {
		return mkInt(int64(hashkey.Sum64([]byte(m.hashDisplay(a0)))), 64)
	}
	return nil
}
