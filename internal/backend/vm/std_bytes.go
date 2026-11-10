package vm

// bytesStd implements Bytes primitives other than concat/tryConcat (kept in
// prelude.go for their Eq/Hash-style naming overlap); nil for other keys.
func (m *Machine) bytesStd(key string, a0, a1, a2 *obj) *obj {
	switch key {
	case "prelude.Bytes.tryZeros", "prelude.ByteBuffer.tryNew":
		if uint64(a0.i) >= maxVMAlloc || m.planRefused(key) {
			return m.allocError()
		}
		return m.okPlanned(key, mkBytes(make([]byte, a0.i)))
	// Go's append never refuses a real, already-in-memory input the way the
	// growBy threshold simulates a refusal for a forward reservation.
	case "prelude.Bytes.tryFromArray":
		if m.planRefused(key) {
			return m.allocError()
		}
		out := make([]byte, len(a0.fields))
		for i, x := range a0.fields {
			out[i] = byte(deref(x).i)
		}
		return m.okPlanned(key, mkBytes(out))
	case "prelude.Bytes.get":
		if a1.i < 0 || a1.i >= int64(len(a0.str())) {
			return m.none()
		}
		return m.some(mkInt(int64(a0.str()[a1.i]), 8))
	}
	return nil
}
