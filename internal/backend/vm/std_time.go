package vm

import (
	"math"
	"time"

	"kigumi/internal/compiler/sem"
)

// timeMonotonicStart anchors time.monotonic(): time.Since keeps using the
// monotonic reading Go's time.Now carries, immune to wall-clock adjustments.
var timeMonotonicStart = time.Now()

func sleepDuration(millis int64) time.Duration {
	if millis > math.MaxInt64/int64(time.Millisecond) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(millis) * time.Millisecond
}

func (m *Machine) registerTime() {
	h := m.host
	h["time.now"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkRecord(m.r.Types.Node(m.retType(fn)).Ent, []*obj{mkInt(time.Now().UnixMilli(), m.numKind(sem.TyI64))})
	}
	h["time.sleep"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		time.Sleep(sleepDuration(deref(a[0]).i))
		return unitObj
	}
	h["time.monotonic"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkRecord(m.r.Types.Node(m.retType(fn)).Ent, []*obj{mkInt(time.Since(timeMonotonicStart).Milliseconds(), m.numKind(sem.TyI64))})
	}
}
