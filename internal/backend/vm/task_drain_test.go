package vm

import "testing"

// Joining, not signalling: a pool whose handle is still held is joined by nothing else.
func TestDrainPoolsJoinsHeldPool(t *testing.T) {
	m := &Machine{sh: newShared()}
	e := m.newExecutor(3)
	m.startWorkers(e)
	m.drainPools()
	m.sh.sched.Lock()
	left := len(m.sh.pools)
	m.sh.sched.Unlock()
	if left != 0 {
		t.Fatalf("%d pools still registered after drainPools", left)
	}
	e.wg.Wait()
}

func TestStopPoolsLeavesPoolRegistered(t *testing.T) {
	m := &Machine{sh: newShared()}
	e := m.newExecutor(2)
	m.startWorkers(e)
	pools := m.stopPools()
	if len(pools) != 1 || pools[0] != e {
		t.Fatalf("stopPools returned %d pools, want the one started", len(pools))
	}
	e.wg.Wait()
	m.forgetPool(e)
}
