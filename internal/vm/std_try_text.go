package vm

import (
	"bytes"
	"unicode/utf8"
)

// tryText runs the fallible text and join primitives. Like tryFamily it spends one fault tick per call, up front, so a refusal leaves nothing built; nil for other keys.
func (m *Machine) tryText(key string, a []*obj, a0, a1, a2 *obj) *obj {
	switch key {
	case "prelude.String.tryTrim", "prelude.String.tryToUpper", "prelude.String.tryToLower", "prelude.String.tryChars", "prelude.String.trySplit",
		"text.tryLines", "text.tryGraphemes", "text.tryReplace", "text.tryTrimPrefix", "text.tryTrimSuffix",
		"array.tryJoin", "array.trySortedBy", "text.StringBuilder.tryPush", "text.StringBuilder.tryPushByte", "text.StringBuilder.tryFreeze", "prelude.tryDebug":
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
	case "prelude.String.tryTrim":
		return m.ok(mkStr(append([]byte{}, bytes.Trim(a0.str(), " \t\r\n")...)))
	case "prelude.String.tryToUpper":
		return m.ok(mkStr(shiftAscii(a0.str(), 'a', 'z', -32)))
	case "prelude.String.tryToLower":
		return m.ok(mkStr(shiftAscii(a0.str(), 'A', 'Z', 32)))
	case "prelude.String.tryChars":
		return m.ok(m.sized(mkArray(charObjs(a0.str()))))
	case "prelude.String.trySplit":
		return m.ok(m.sized(mkArray(m.splitPieces(a0, a1))))
	case "text.tryLines":
		return m.ok(m.sized(mkArray(linePieces(a0.str()))))
	case "text.tryGraphemes":
		return m.ok(m.sized(mkArray(graphemePieces(a0.str()))))
	case "text.tryReplace":
		if len(a1.str()) == 0 {
			return m.ok(retain(a0))
		}
		return m.ok(mkStr(bytes.ReplaceAll(a0.str(), a1.str(), a2.str())))
	case "text.tryTrimPrefix":
		if len(a1.str()) == 0 || !bytes.HasPrefix(a0.str(), a1.str()) {
			return m.ok(retain(a0))
		}
		return m.ok(mkStr(append([]byte{}, a0.str()[len(a1.str()):]...)))
	case "text.tryTrimSuffix":
		if len(a1.str()) == 0 || !bytes.HasSuffix(a0.str(), a1.str()) {
			return m.ok(retain(a0))
		}
		return m.ok(mkStr(append([]byte{}, a0.str()[:len(a0.str())-len(a1.str())]...)))
	case "array.tryJoin":
		parts := make([][]byte, len(a0.fields))
		for i, x := range a0.fields {
			parts[i] = deref(x).str()
		}
		return m.ok(mkStr(bytes.Join(parts, a1.str())))
	case "array.trySortedBy":
		out := copyItems(m, a0, 0, int64(len(a0.fields)), false)
		m.mergeSort(a1, out.fields, make([]*obj, len(out.fields)))
		return m.ok(out)
	case "text.StringBuilder.tryPushByte":
		arr := deref(a0.fields[0])
		arr.fields = append(arr.fields, mkInt(a1.i&255, 8))
		m.chargeMemory(arr)
		return m.ok(unitObj)
	case "text.StringBuilder.tryPush":
		arr := deref(a0.fields[0])
		for _, b := range a1.str() {
			arr.fields = append(arr.fields, mkInt(int64(b), 8))
		}
		m.chargeMemory(arr)
		return m.ok(unitObj)
	}
	return m.ok(mkStr([]byte(m.display(a0))))
}
