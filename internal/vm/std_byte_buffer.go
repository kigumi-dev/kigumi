package vm

import "unicode/utf8"

// byteBuffer implements prelude.ByteBuffer, a Bytes object written in place; tryNew is Bytes.tryZeros. Nil for other keys.
func (m *Machine) byteBuffer(key string, a []*obj) *obj {
	switch key {
	case "prelude.ByteBuffer.set":
		buf, at := deref(a[0]), deref(a[1]).i
		if at < 0 || at >= int64(len(buf.str())) {
			m.abort("ByteBuffer.set: index out of range")
		}
		buf.str()[at] = byte(deref(a[2]).i)
		return unitObj
	case "prelude.ByteBuffer.copyFrom", "prelude.ByteBuffer.copyFromString":
		buf, at, src := deref(a[0]), deref(a[1]).i, deref(a[2]).str()
		lo, hi := deref(a[3]).i, deref(a[4]).i
		if lo < 0 || lo > hi || hi > int64(len(src)) || at < 0 || at > int64(len(buf.str()))-(hi-lo) {
			m.abort("ByteBuffer.copyFrom: range out of bounds")
		}
		copy(buf.str()[at:], src[lo:hi])
		return unitObj
	case "prelude.ByteBuffer.freeze":
		return retain(deref(a[0]))
	case "prelude.ByteBuffer.freezeString":
		buf := deref(a[0])
		if !utf8.Valid(buf.str()) {
			m.abort("ByteBuffer.freezeString: invalid UTF-8")
		}
		buf.k = kStr
		return retain(buf)
	}
	return nil
}
