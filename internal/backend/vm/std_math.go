package vm

import (
	"math"

	"kigumi/internal/compiler/sem"
)

// mathPrims are the std/math primitives, each a single machine operation on native.
var mathPrims = map[string]func(a *obj) *obj{
	"math.sqrt":     func(a *obj) *obj { return mkFloat(math.Sqrt(a.fl()), 64|512) },
	"math.floor":    func(a *obj) *obj { return mkFloat(rounded(math.Floor, a.fl()), 64|512) },
	"math.ceil":     func(a *obj) *obj { return mkFloat(rounded(math.Ceil, a.fl()), 64|512) },
	"math.round":    func(a *obj) *obj { return mkFloat(rounded(math.Round, a.fl()), 64|512) },
	"math.toBits":   func(a *obj) *obj { return mkInt(int64(math.Float64bits(a.fl())), 64) },
	"math.fromBits": func(a *obj) *obj { return mkFloat(math.Float64frombits(uint64(a.i)), 64|512) },
}

// registerMath puts the primitives in the host table, so a planned call reaches one without std's key switches.
func (m *Machine) registerMath() {
	for key, f := range mathPrims {
		m.host[key] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
			return m.stdResultTaint(key, a, f(deref(a[0])))
		}
	}
}

// mathFn implements the std/math primitives; nil for other keys.
func (m *Machine) mathFn(key string, a0 *obj) *obj {
	if f, ok := mathPrims[key]; ok {
		return f(a0)
	}
	return nil
}

// rounded returns a NaN argument quiet with its payload on every CPU; Go's Floor and Ceil quiet a signaling NaN only where
// they use ROUNDSD.
func rounded(f func(float64) float64, x float64) float64 {
	if x != x {
		return math.Float64frombits(math.Float64bits(x) | 1<<51)
	}
	return f(x)
}
