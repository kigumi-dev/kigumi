package vm

import (
	"bytes"

	"kigumi/internal/compiler/sem"
)

// registerTextAccel installs Go accelerators for the std text bodies the VM runs too slowly; NoAccel runs the bodies.
func (m *Machine) registerTextAccel() {
	// The bodies' allocation sequence is the one native follows, so a fault sweep must run them.
	if m.opts.FaultInjectAt > 0 {
		return
	}
	h := m.host
	str := func(a []*obj, i int) []byte { return deref(a[i]).str() }
	strOk := func(m *Machine, s []byte) *obj { return m.ok(mkStr(s)) }
	h["prelude.String.tryTrim"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return strOk(m, append([]byte{}, bytes.Trim(str(a, 0), " \t\r\n")...))
	}
	h["prelude.String.tryToUpper"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return strOk(m, shiftAscii(str(a, 0), 'a', 'z', -32))
	}
	h["prelude.String.tryToLower"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return strOk(m, shiftAscii(str(a, 0), 'A', 'Z', 32))
	}
	h["prelude.String.trySplit"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return m.ok(m.sized(mkArray(m.splitPieces(deref(a[0]), deref(a[1])))))
	}
	h["text.tryLines"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return m.ok(m.sized(mkArray(linePieces(str(a, 0)))))
	}
	h["text.tryScalars"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return m.ok(m.sized(mkArray(graphemePieces(str(a, 0)))))
	}
	h["text.tryReplace"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		s, old, repl := str(a, 0), str(a, 1), str(a, 2)
		if len(old) == 0 {
			return m.ok(retain(deref(a[0])))
		}
		n := bytes.Count(s, old)
		if uint64(len(s)-n*len(old)+n*len(repl)) >= maxVMAlloc {
			return m.allocError()
		}
		return strOk(m, bytes.ReplaceAll(s, old, repl))
	}
	h["text.tryTrimPrefix"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		s, p := str(a, 0), str(a, 1)
		if len(p) == 0 || !bytes.HasPrefix(s, p) {
			return m.ok(retain(deref(a[0])))
		}
		return strOk(m, append([]byte{}, s[len(p):]...))
	}
	h["text.tryTrimSuffix"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		s, p := str(a, 0), str(a, 1)
		if len(p) == 0 || !bytes.HasSuffix(s, p) {
			return m.ok(retain(deref(a[0])))
		}
		return strOk(m, append([]byte{}, s[:len(s)-len(p)]...))
	}
	h["array.tryJoin"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		items := deref(a[0]).fields
		parts := make([][]byte, len(items))
		total := uint64(0)
		for i, x := range items {
			parts[i] = deref(x).str()
			total += uint64(len(parts[i]))
		}
		if len(items) > 1 {
			total += uint64(len(items)-1) * uint64(len(str(a, 1)))
		}
		if total >= maxVMAlloc {
			return m.allocError()
		}
		return strOk(m, bytes.Join(parts, str(a, 1)))
	}
	h["text.StringBuilder.tryPush"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		arr := deref(deref(a[0]).fields[0])
		for _, b := range str(a, 1) {
			arr.fields = append(arr.fields, mkInt(int64(b), 8))
		}
		m.chargeMemory(arr)
		return m.ok(unitObj)
	}
	h["prelude.String.contains"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return mkBool(bytes.Contains(str(a, 0), str(a, 1)))
	}
	index := func(m *Machine, at int) *obj {
		if at < 0 {
			return m.none()
		}
		return m.some(mkInt(int64(at), m.numKind(sem.TyUsize)))
	}
	h["prelude.Bytes.indexOf"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return index(m, bytes.Index(str(a, 0), str(a, 1)))
	}
	h["prelude.Bytes.lastIndexOf"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		return index(m, bytes.LastIndex(str(a, 0), str(a, 1)))
	}
}
