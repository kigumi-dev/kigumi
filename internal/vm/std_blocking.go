package vm

import (
	"sync/atomic"

	"kigumi/internal/sem"
)

// blockingPool bounds admitted-but-unclaimed BlockingPool.run results; the
// Future it hands back can be taken or dropped on another lane, so count atomically.
type blockingPool struct {
	capacity int
	inFlight atomic.Int64
}

func (m *Machine) registerBlocking() {
	h := m.host
	h["task.BlockingPool.new"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkOpaque("BlockingPool", &blockingPool{capacity: int(deref(a[0]).i)})
	}
	h["task.BlockingPool.run"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		pool := deref(a[0]).data.(*blockingPool)
		if pool.inFlight.Load() >= int64(pool.capacity) {
			return m.blockingCapacityExhausted()
		}
		r := m.callValue(a[1], nil)
		pool.inFlight.Add(1)
		return m.ok(mkOpaque("Future", &future{result: r, pool: retain(deref(a[0]))}))
	}
}

func (m *Machine) blockingCapacityExhausted() *obj {
	e := mkVariant(m.variantOf(m.lookup("std/task", "BlockingError"), "CapacityExhausted"), nil)
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{e})
}
