package vm

import (
	"fmt"
	"runtime/debug"
)

// objOverhead approximates one obj allocation's fixed bookkeeping cost.
const objOverhead = 32

// maxVMAlloc bounds one make() request, because Go's out-of-memory error is
// fatal and cannot be recovered; fixed at 4 GiB, lower under GOMEMLIMIT.
var maxVMAlloc = func() uint64 {
	limit := uint64(1) << 32
	if mem := debug.SetMemoryLimit(-1); mem > 0 && uint64(mem) < limit {
		limit = uint64(mem)
	}
	return limit
}()

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
	case kArray:
		// An array slot always holds one pointer; a scalar element also holds
		// the object that pointer names, charged per slot even when the VM
		// interns it. Aggregate elements charge their own object instead.
		n := int64(len(v.fields))*8 + objOverhead
		for _, f := range v.fields {
			if f == nil {
				continue
			}
			switch f.k {
			case kInt, kFloat, kChar:
				n += objOverhead
			}
		}
		return n
	case kRecord, kVariant, kClosure:
		// Record fields share the same slot layout; only a fresh scalar adds
		// a boxed object, while an interned scalar is already accounted.
		n := int64(len(v.fields))*8 + objOverhead
		for _, f := range v.fields {
			if f == nil || f.rc < 0 {
				continue
			}
			switch f.k {
			case kInt, kFloat, kChar:
				n += objOverhead
			}
		}
		return n
	case kRawArray:
		return int64(len(v.rawArr().raw))*8 + objOverhead
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
	if m.trace != nil {
		m.trace.alloc(v)
	}
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
		m.limitAbort(m.memoryLimitText())
	}
}

// reserveMemory aborts a sandboxed run before it allocates n bytes the memory limit would refuse.
func (m *Machine) reserveMemory(n uint64) {
	if !m.opts.Sandbox || m.opts.MemoryLimit <= 0 {
		return
	}
	if n > uint64(m.opts.MemoryLimit) || m.sh.mem.Load()+int64(n) > m.opts.MemoryLimit {
		m.limitAbort(m.memoryLimitText())
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

// allocHeader takes the one allocation native's rt_opaque charges for a
// runtime header it hands to Kigumi code (an allocator scope guard, a Future,
// a Task, an Executor). The op that creates the header has no Err form to
// report a refusal, so it is fatal, exactly as native's alloc_in/rt_opaque
// panic ("out of memory") through rt_panic. Callers run this before retaining
// anything the header owns, so a refusal leaks no reference.
func (m *Machine) allocHeader() {
	if m.faultTick() {
		m.abort("out of memory")
	}
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
			m.limitAbort(fmt.Sprintf("step limit of %d steps", m.opts.StepLimit))
		}
		if m.sh.steps.CompareAndSwap(used, used+n) {
			m.stepsLeft = int(n) - 1
			return
		}
	}
}

func (m *Machine) memoryLimitText() string {
	return fmt.Sprintf("memory limit of %d MiB", m.opts.MemoryLimit>>20)
}
