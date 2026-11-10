package vm

import "fmt"

// diff compares two runs and names the first difference.
func (t *traceLog) diff(o *traceLog) error {
	if t == nil || o == nil {
		return nil
	}
	n := min(len(t.blocks), len(o.blocks))
	for i := range t.blocks[:n] {
		a, b := t.blocks[i], o.blocks[i]
		if a != b {
			return fmt.Errorf("SecretSan: trace mismatch in executed blocks at step %d: %s#%d vs %s#%d", i, a.fn, a.blk, b.fn, b.blk)
		}
	}
	if len(t.blocks) != len(o.blocks) {
		return fmt.Errorf("SecretSan: trace mismatch in executed blocks: %d steps vs %d", len(t.blocks), len(o.blocks))
	}
	n = min(len(t.indices), len(o.indices))
	for i := range t.indices[:n] {
		a, b := t.indices[i], o.indices[i]
		if a != b {
			return fmt.Errorf("SecretSan: trace mismatch in object-relative indices at step %d: %s %s[%d] vs %s %s[%d]",
				i, a.op, kindName(a.kind), a.index, b.op, kindName(b.kind), b.index)
		}
	}
	if len(t.indices) != len(o.indices) {
		return fmt.Errorf("SecretSan: trace mismatch in object-relative indices: %d accesses vs %d", len(t.indices), len(o.indices))
	}
	n = min(len(t.allocs), len(o.allocs))
	for i := range t.allocs[:n] {
		a, b := t.allocs[i], o.allocs[i]
		if a != b {
			return fmt.Errorf("SecretSan: trace mismatch in allocations at step %d: %s of %d bytes vs %s of %d bytes",
				i, kindName(a.kind), a.size, kindName(b.kind), b.size)
		}
	}
	if len(t.allocs) != len(o.allocs) {
		return fmt.Errorf("SecretSan: trace mismatch in allocations: %d events vs %d", len(t.allocs), len(o.allocs))
	}
	n = min(len(t.rc), len(o.rc))
	for i := range t.rc[:n] {
		a, b := t.rc[i], o.rc[i]
		if a != b {
			return fmt.Errorf("SecretSan: trace mismatch in RC operations at step %d: %s %s vs %s %s",
				i, a.op, kindName(a.kind), b.op, kindName(b.kind))
		}
	}
	if len(t.rc) != len(o.rc) {
		return fmt.Errorf("SecretSan: trace mismatch in RC operations: %d operations vs %d", len(t.rc), len(o.rc))
	}
	return nil
}
