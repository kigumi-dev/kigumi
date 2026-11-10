package vm

import (
	"sync/atomic"

	"kigumi/internal/compiler/sem"
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
		m.allocHeader()
		return mkOpaque("BlockingPool", &blockingPool{capacity: int(deref(a[0]).i)})
	}
	h["task.BlockingPool.run"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		pool := deref(a[0]).opaqueData().(*blockingPool)
		if !reserveBlocking(pool) {
			return m.blockingCapacityExhausted()
		}
		if m.cur != nil && m.cur.ex.workers > 0 {
			return m.ok(m.offload(deref(a[0]), retain(deref(a[1]))))
		}
		r := m.callValue(a[1], nil)
		m.allocHeader()
		return m.ok(mkOpaque("Future", &future{result: r, pool: retain(deref(a[0]))}))
	}
}

func (m *Machine) blockingCapacityExhausted() *obj {
	e := mkVariant(m.variantOf(m.lookup("std/task", "BlockingError"), "CapacityExhausted"), nil)
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{e})
}

func reserveBlocking(p *blockingPool) bool {
	for {
		cur := p.inFlight.Load()
		if cur >= int64(p.capacity) {
			return false
		}
		if p.inFlight.CompareAndSwap(cur, cur+1) {
			return true
		}
	}
}
