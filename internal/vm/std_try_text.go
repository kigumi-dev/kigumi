package vm

import "unicode/utf8"

// tryText runs the fallible text primitives that are not Kigumi bodies. Like tryFamily it spends one fault tick per call, up front, so a refusal leaves nothing built; nil for other keys.
func (m *Machine) tryText(key string, a []*obj, a0, a1, a2 *obj) *obj {
	switch key {
	case "prelude.String.tryChars", "array.trySortedBy", "text.StringBuilder.tryFreeze", "prelude.tryDebug":
	default:
		return nil
	}
	if m.faultTick() {
		if key == "text.StringBuilder.tryFreeze" {
			return m.errMsg("out of memory")
		}
		return m.allocError()
	}
	switch key {
	case "text.StringBuilder.tryFreeze":
		raw := make([]byte, 0, len(deref(a0.fields[0]).fields))
		for _, x := range deref(a0.fields[0]).fields {
			raw = append(raw, byte(deref(x).i))
		}
		if !utf8.Valid(raw) {
			return m.errMsg("invalid UTF-8")
		}
		return m.ok(mkStr(raw))
	case "prelude.String.tryChars":
		return m.ok(m.sized(mkArray(charObjs(a0.str()))))
	case "array.trySortedBy":
		out := copyItems(m, a0, 0, int64(len(a0.fields)), false)
		m.mergeSort(a1, out.fields, make([]*obj, len(out.fields)))
		return m.ok(out)
	}
	return m.ok(mkStr([]byte(m.display(a0))))
}
