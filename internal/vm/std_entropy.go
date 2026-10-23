package vm

import (
	"crypto/rand"
	"encoding/binary"

	"kigumi/internal/sem"
)

// std/entropy's real body reaches getrandom(2) through extern(C), which the
// VM cannot call; crypto/rand gives the same guarantee here.
func (m *Machine) registerEntropy() {
	h := m.host
	h["entropy.Entropy.bytes"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		n := deref(a[1]).i
		if n < 0 || uint64(n) >= maxVMAlloc {
			return m.errMsg("out of memory")
		}
		buf := make([]byte, n)
		if _, err := rand.Read(buf); err != nil {
			return m.errMsg(err.Error())
		}
		return m.ok(mkBytes(buf))
	}
	h["entropy.Entropy.u64"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return m.errMsg(err.Error())
		}
		v := binary.BigEndian.Uint64(buf[:])
		clear(buf[:])
		return m.ok(mkInt(int64(v), m.numKind(sem.TyU64)))
	}
}
