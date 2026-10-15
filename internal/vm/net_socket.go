package vm

import (
	"kigumi/internal/netsock"
	"kigumi/internal/sem"
)

// std/net's Conn/Listener bodies reach the host through extern(C) socket
// calls the VM cannot make; these are their Go counterparts, built on netsock's raw-fd implementation.
func (m *Machine) registerNet() {
	h := m.host
	fdOf := func(v *obj) int { return int(deref(deref(v).fields[0]).i) }
	connRecord := func(fn sem.EntityID, fd int) *obj {
		ent := m.r.Types.Node(m.resultPayload(fn)).Ent
		return mkRecord(ent, []*obj{mkInt(int64(fd), m.numKind(sem.TyI32))})
	}
	dialOrListen := func(dial func(string, int64) (int, error)) hostFn {
		return func(m *Machine, fn sem.EntityID, a []*obj) *obj {
			host := string(deref(a[1]).str())
			port := deref(a[2]).i
			fd, err := dial(host, port)
			if err != nil {
				return m.errMsg(err.Error())
			}
			return m.ok(connRecord(fn, fd))
		}
	}
	h["net.Net.connect"] = dialOrListen(netsock.Dial)
	h["net.Net.listen"] = dialOrListen(netsock.Listen)
	h["net.Listener.accept"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		fd, err := netsock.Accept(fdOf(a[0]))
		if err != nil {
			return m.errMsg(err.Error())
		}
		return m.ok(connRecord(fn, fd))
	}
	h["net.Listener.port"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkInt(int64(netsock.Port(fdOf(a[0]))), m.numKind(sem.TyI64))
	}
	h["net.Conn.read"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		max := deref(a[2]).i
		if max < 0 {
			return m.errMsg("out of memory")
		}
		buf := make([]byte, max)
		got, err := netsock.Read(fdOf(a[0]), buf)
		if err != nil {
			return m.errMsg(err.Error())
		}
		arr := deref(a[1])
		for _, b := range buf[:got] {
			arr.fields = append(arr.fields, mkInt(int64(b), 8))
		}
		return m.ok(mkInt(int64(got), m.numKind(sem.TyUsize)))
	}
	h["net.Conn.write"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		sent, err := netsock.Write(fdOf(a[0]), deref(a[1]).str())
		if err != nil {
			return m.errMsg(err.Error())
		}
		return m.ok(mkInt(int64(sent), m.numKind(sem.TyUsize)))
	}
	h["net.Conn.close"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return m.ok(unitObj)
	}
	release := func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		releaseSocket(a[0])
		return unitObj
	}
	h["net.Conn.drop"] = release
	h["net.Listener.drop"] = release
}

// releaseSocket closes the record's fd once; a repeat would close whatever another goroutine has since opened under the same number.
func releaseSocket(v *obj) {
	r := deref(v)
	if fd := deref(r.fields[0]); fd.i >= 0 {
		netsock.Close(int(fd.i))
		rewriteInt(r, 0, -1)
	}
}
