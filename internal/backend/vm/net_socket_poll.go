package vm

import (
	hostnet "kigumi/internal/backend/vm/host/net"
	"kigumi/internal/compiler/sem"
)

func (m *Machine) registerNetPoll() {
	h := m.host
	fdOf := func(v *obj) int { return int(deref(deref(v).fields[0]).i) }
	h["net.Listener.pollAccept"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		fd, ok, err := hostnet.TryAccept(fdOf(a[0]))
		if err != nil {
			return m.errMsg(ioError(err).Error())
		}
		if !ok {
			return m.ok(m.none())
		}
		conn, _ := m.r.Types.IsOption(m.resultPayload(fn))
		ent := m.r.Types.Node(conn).Ent
		return m.ok(m.some(mkRecord(ent, []*obj{mkInt(int64(fd), m.numKind(sem.TyI32))})))
	}
	h["net.Conn.pollRead"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		max := deref(a[2]).i
		if max < 0 || uint64(max) >= maxVMAlloc {
			return m.errMsg("out of memory")
		}
		if max == 0 {
			return m.ok(m.some(mkInt(0, m.numKind(sem.TyUsize))))
		}
		buf := make([]byte, max)
		got, ok, err := hostnet.TryRead(fdOf(a[0]), buf)
		if err != nil {
			return m.errMsg(ioError(err).Error())
		}
		if !ok {
			return m.ok(m.none())
		}
		arr := deref(a[1])
		for _, b := range buf[:got] {
			arr.fields = append(arr.fields, mkInt(int64(b), 8))
		}
		return m.ok(m.some(mkInt(int64(got), m.numKind(sem.TyUsize))))
	}
	h["net.Conn.pollWrite"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		data := deref(a[1]).str()
		if len(data) == 0 {
			return m.ok(m.some(mkInt(0, m.numKind(sem.TyUsize))))
		}
		sent, ok, err := hostnet.TryWrite(fdOf(a[0]), data)
		if err != nil {
			return m.errMsg(ioError(err).Error())
		}
		if !ok {
			return m.ok(m.none())
		}
		return m.ok(m.some(mkInt(int64(sent), m.numKind(sem.TyUsize))))
	}
}
