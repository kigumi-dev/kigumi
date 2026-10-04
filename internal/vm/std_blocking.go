package vm

import "kigumi/internal/sem"

// blockingPool is a BlockingPool: a root-owned, bounded count of blockingFuture results admitted but not taken.
// run executes work on the calling goroutine, not a separate one, so inFlight is a plain int, not atomic.
type blockingPool struct {
	capacity, inFlight int
}

// blockingFuture is one admitted BlockingPool.run result; pool is retained
// so the pool outlives every future it issued.
type blockingFuture struct {
	pool, result *obj
	taken        bool
}

func (m *Machine) registerBlocking() {
	h := m.host
	h["task.BlockingPool.new"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkOpaque("BlockingPool", &blockingPool{capacity: int(deref(a[0]).i)})
	}
	h["task.BlockingPool.run"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		pool := deref(a[0]).data.(*blockingPool)
		if pool.inFlight >= pool.capacity {
			return m.blockingCapacityExhausted()
		}
		r := m.callValue(a[1], nil)
		pool.inFlight++
		return m.ok(mkOpaque("BlockingFuture", &blockingFuture{pool: retain(deref(a[0])), result: r}))
	}
}

func (m *Machine) blockingCapacityExhausted() *obj {
	e := mkVariant(m.variantOf(m.lookup("std/task", "BlockingError"), "CapacityExhausted"), nil)
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{e})
}

func (m *Machine) blockingFutureJoin(f *blockingFuture) *obj {
	f.taken = true
	f.pool.data.(*blockingPool).inFlight--
	return f.result
}
