package vm

// resultRule says how the arguments of a primitive or builtin reach its result.
type resultRule uint8

const (
	// resultAll: content and shape follow every argument, as the callee may branch on any input.
	resultAll resultRule = iota
	// resultLen: a length, read from argument 0's shape (or the whole of a range, whose bounds are its content).
	resultLen
	// resultShapes: content follows every argument; the shape (a length, or a Result tag only an allocation decides)
	// follows the arguments' shapes and their index or size arguments, not their contents.
	resultShapes
	// resultDeep: like resultAll over everything reachable from the arguments.
	resultDeep
	resultNone
	// resultSlot: a static or MMIO slot read whose stored value carries its own label.
	resultSlot
)

// primResultTable lists the primitives more precise than resultAll. Every fallible allocation whose Result tag
// depends only on sizes is here; one whose output length follows the contents (trim, split, join) keeps resultAll.
var primResultTable = map[string]resultRule{
	"array.Array.len":                 resultLen,
	"prelude.String.len":              resultLen,
	"prelude.Bytes.len":               resultLen,
	"array.Array.of":                  resultShapes,
	"array.Array.get":                 resultShapes,
	"array.Array.first":               resultShapes,
	"array.Array.last":                resultShapes,
	"array.Array.with":                resultShapes,
	"array.Array.withMut":             resultShapes,
	"array.Array.takeAt":              resultShapes,
	"array.Array.trySlice":            resultShapes,
	"array.Array.tryCopyRange":        resultShapes,
	"array.Array.tryReverse":          resultShapes,
	"prelude.FixedArray.adopt":        resultShapes,
	"prelude.FixedArray.withMut":      resultShapes,
	"prelude.String.toBytes":          resultShapes,
	"prelude.String.bytes":            resultShapes,
	"prelude.Bytes.get":               resultShapes,
	"prelude.Bytes.trySlice":          resultShapes,
	"prelude.Bytes.tryFromArray":      resultShapes,
	"prelude.ByteBuffer.freeze":       resultShapes,
	"prelude.ByteBuffer.freezeString": resultShapes,
	"alloc.Shared.tryGet":             resultShapes,
	"alloc.Shared.tryNew":             resultShapes,
	"alloc.Shared.tryDowngrade":       resultShapes,
	"alloc.Box.tryNew":                resultShapes,
	"alloc.Arena.tryCreate":           resultShapes,
	"array.Array.tryConcat":           resultShapes,
	"array.Array.tryFilter":           resultShapes,
	"array.Array.tryInsertAt":         resultShapes,
	"array.Array.tryMap":              resultShapes,
	"array.Array.tryWithCapacity":     resultShapes,
	"array.trySortedBy":               resultShapes,
	"map.Map.tryEntries":              resultShapes,
	"map.tryEmpty":                    resultShapes,
	"mem.trySecureErase":              resultShapes,
	"prelude.AllocError.wrap":         resultShapes,
	"prelude.ByteBuffer.tryNew":       resultShapes,
	"prelude.Bytes.tryConcat":         resultShapes,
	"prelude.Bytes.tryZeros":          resultShapes,
	"text.StringBuilder.tryNew":       resultShapes,
	"text.StringBuilder.tryPush":      resultShapes,
	"map.Map.tryClone":                resultShapes,
	"os.OsString.tryClone":            resultShapes,
	"prelude.debug":                   resultDeep,
}

// labelResult applies rule to r, the result of a call on args whose integer arguments have roles.
func (m *Machine) labelResult(rule resultRule, roles []primArg, args []*obj, r *obj) *obj {
	if !m.opts.SecretSan {
		return r
	}
	switch rule {
	case resultNone, resultSlot:
		return r
	case resultLen:
		if len(args) > 0 && m.lengthTainted(args[0]) {
			return m.withTaint(r, fullShadow(64))
		}
		return r
	case resultDeep:
		t := m.deepTaint(args)
		return m.withLabel(r, label{elem: t, shape: t.tainted()})
	case resultShapes:
		l := label{elem: m.joinTaint(args), shape: m.anyShape(args)}
		for _, a := range roles {
			if a.role != roleValue && a.at < len(args) && m.taintOf(args[a.at]).tainted() {
				l.shape = true
			}
		}
		return m.withLabel(r, l)
	}
	t := m.joinTaint(args)
	return m.withLabel(r, label{elem: t, shape: t.tainted()})
}

// lengthTainted reports whether the length of v depends on a secret: a container's shape, a map's key list, or a range's bounds.
func (m *Machine) lengthTainted(v *obj) bool {
	d := derefRaw(v)
	switch {
	case d == nil:
		return false
	case d.k == kArray || d.k == kRawArray || d.k == kBytes || d.k == kStr:
		return m.shapeOf(v)
	case d.k == kRecord && d.ent == m.r.MapEntity() && len(d.fields) > 0:
		return m.shapeOf(v) || m.shapeOf(d.fields[0])
	}
	return m.taintOf(v).tainted()
}

// stdResultTaint labels a primitive's result by its rule; select is bit-precise.
func (m *Machine) stdResultTaint(key string, args []*obj, r *obj) *obj {
	if !m.opts.SecretSan {
		return r
	}
	if key == "secret.selectU64" && len(args) >= 3 && m.taintOf(args[0]).tainted() {
		return m.withTaint(r, m.selectShadow(args[0], args[1], args[2]))
	}
	return m.labelResult(primResultTable[key], primArgTable[key], args, r)
}
