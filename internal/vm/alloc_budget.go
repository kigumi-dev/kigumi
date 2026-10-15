package vm

// objOverhead approximates one obj allocation's fixed bookkeeping cost.
const objOverhead = 32

// maxVMAlloc bounds one make() request; past it a size-taking primitive
// reports AllocError instead of risking Go's own makeslice panic.
const maxVMAlloc = 1 << 40

// approxSize estimates the bytes a value's own backing storage costs right
// now. Scalars (int/float/bool/char/unit) count as 0: the sandbox's memory
// budget bounds growing data (strings, bytes, arrays, records, closures),
// not the interpreter's per-instruction temporaries, so a tight arithmetic
// loop still fails on comptimeStepLimit first (frame.go), matching the
// sandbox's separate step/memory/depth axes.
func approxSize(v *obj) int64 {
	if v == nil {
		return 0
	}
	switch v.k {
	case kStr, kBytes:
		return int64(len(v.str())) + objOverhead
	case kArray, kRecord, kVariant, kClosure:
		return int64(len(v.fields))*8 + objOverhead
	case kCell, kBox, kOpaque:
		return objOverhead
	}
	return 0
}

// chargeMemory brings v's share of the sandboxed run's live total up to
// date and aborts once it crosses Options.MemoryLimit; 0 (EvalBuild)
// leaves allocation unbounded. v.chg holds the last-charged byte count, not
// a one-shot flag, so re-charging the same size is a no-op and a primitive
// that grows a live receiver in place (Array.push) can bill the growth too.
func (m *Machine) chargeMemory(v *obj) {
	if !m.opts.Sandbox || m.opts.MemoryLimit <= 0 || v == nil {
		return
	}
	size := approxSize(v)
	delta := size - v.charged()
	if delta == 0 {
		return
	}
	v.setCharged(size)
	if live := m.sh.mem.Add(delta); live < 0 {
		m.sh.mem.Store(0)
	} else if delta > 0 && live > m.opts.MemoryLimit {
		m.sandboxAbort("comptime block allocated too much memory")
	}
}

// unchargeMemory reclaims a freed value's last-charged share of the live
// total. The total tracks live bytes, not cumulative churn, so a loop that
// builds and immediately drops small values stays bounded instead of
// tripping the limit on volume alone.
func (m *Machine) unchargeMemory(v *obj) {
	if !m.opts.Sandbox || m.opts.MemoryLimit <= 0 || v == nil || v.charged() == 0 {
		return
	}
	if m.sh.mem.Add(-v.charged()) < 0 {
		m.sh.mem.Store(0)
	}
}

// faultTick counts one allocation attempt and reports whether this is the
// one FaultInjectAt names (1-based); disabled (0) never fires.
func (m *Machine) faultTick() bool {
	if m.opts.FaultInjectAt <= 0 || (m.opts.FaultArmOnEprint && !m.sh.faultArmed.Load()) {
		return false
	}
	return m.sh.faultN.Add(1) == m.opts.FaultInjectAt
}

// stepBatch bounds reservations so the hot loop avoids an atomic per instruction.
const stepBatch = 256

func (m *Machine) chargeStep() {
	if !m.opts.Sandbox {
		return
	}
	if m.stepsLeft > 0 {
		m.stepsLeft--
		return
	}
	m.reserveSteps()
}

// reserveSteps never exceeds what is left, so the budget trips exactly at StepLimit.
func (m *Machine) reserveSteps() {
	limit := int64(m.opts.StepLimit)
	for {
		used := m.sh.steps.Load()
		n := min(stepBatch, limit-used)
		if n <= 0 {
			m.sandboxAbort("comptime block took too long")
		}
		if m.sh.steps.CompareAndSwap(used, used+n) {
			m.stepsLeft = int(n) - 1
			return
		}
	}
}
