package vm

import (
	"bytes"

	"kigumi/internal/sem"
)

// bytesStd implements Bytes primitives other than concat/tryConcat (kept in
// prelude.go for their Eq/Hash-style naming overlap); nil for other keys.
func (m *Machine) bytesStd(key string, a0, a1, a2 *obj) *obj {
	switch key {
	case "prelude.Bytes.indexOf":
		if at := bytes.Index(a0.str(), a1.str()); at >= 0 {
			return m.some(mkInt(int64(at), m.numKind(sem.TyUsize)))
		}
		return m.none()
	case "prelude.Bytes.lastIndexOf":
		if at := bytes.LastIndex(a0.str(), a1.str()); at >= 0 {
			return m.some(mkInt(int64(at), m.numKind(sem.TyUsize)))
		}
		return m.none()
	case "prelude.String.contains":
		return mkBool(bytes.Contains(a0.str(), a1.str()))
	case "prelude.Bytes.slice":
		s, e := a1.i, a2.i
		if s < 0 || s > e || e > int64(len(a0.str())) {
			return m.none()
		}
		return m.some(mkBytes(append([]byte{}, a0.str()[s:e]...)))
	case "prelude.Bytes.zeros":
		if uint64(a0.i) >= maxVMAlloc || m.faultTick() {
			m.abort("out of memory")
		}
		return mkBytes(make([]byte, a0.i))
	case "prelude.Bytes.tryZeros":
		if uint64(a0.i) >= maxVMAlloc || m.faultTick() {
			return m.allocError()
		}
		return m.ok(mkBytes(make([]byte, a0.i)))
	case "prelude.Bytes.fill":
		if uint64(a1.i) >= maxVMAlloc || m.faultTick() {
			m.abort("out of memory")
		}
		out := make([]byte, a1.i)
		for i := range out {
			out[i] = byte(a0.i)
		}
		return mkBytes(out)
	case "prelude.Bytes.tryFill":
		if uint64(a1.i) >= maxVMAlloc || m.faultTick() {
			return m.allocError()
		}
		out := make([]byte, a1.i)
		for i := range out {
			out[i] = byte(a0.i)
		}
		return m.ok(mkBytes(out))
	case "prelude.Bytes.fromArray":
		if m.faultTick() {
			m.abort("out of memory")
		}
		out := make([]byte, len(a0.fields))
		for i, x := range a0.fields {
			out[i] = byte(deref(x).i)
		}
		return mkBytes(out)
	// Go's append never refuses a real, already-in-memory input the way the
	// growBy threshold simulates a refusal for a forward reservation.
	case "prelude.Bytes.tryFromArray":
		if m.faultTick() {
			return m.allocError()
		}
		out := make([]byte, len(a0.fields))
		for i, x := range a0.fields {
			out[i] = byte(deref(x).i)
		}
		return m.ok(mkBytes(out))
	case "prelude.Bytes.get":
		if a1.i < 0 || a1.i >= int64(len(a0.str())) {
			return m.none()
		}
		return m.some(mkInt(int64(a0.str()[a1.i]), 8))
	}
	return nil
}
