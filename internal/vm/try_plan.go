package vm

import "kigumi/internal/mir"

type resultCopy uint8

const (
	copyValue resultCopy = iota
	copySome
	copyOk
	copyElements
)

type tryPlan struct {
	own, layers int
	copy        resultCopy
}

var tryPlans = map[string]tryPlan{
	"prelude.Bytes.tryZeros":       {own: 2, layers: 1},
	"prelude.ByteBuffer.tryNew":    {own: 2, layers: 1},
	"prelude.Bytes.tryFromArray":   {own: 2, layers: 1},
	"prelude.Bytes.tryConcat":      {own: 2, layers: 1},
	"prelude.String.trySliceBytes": {own: 2, layers: 2},
	"prelude.Bytes.trySlice":       {own: 2, layers: 2},
	"alloc.Shared.tryNew":          {own: 3, layers: 1},
	"alloc.Shared.tryDowngrade":    {own: 1, layers: 1},
	"alloc.Box.tryNew":             {own: 2, layers: 1},
	"alloc.Shared.tryGet":          {layers: 1, copy: copyOk},
	"map.Map.tryClone":             {layers: 1, copy: copyOk},
	"os.OsString.tryClone":         {layers: 1, copy: copyOk},
	"array.Array.get":              {copy: copySome},
	"array.Array.first":            {copy: copySome},
	"array.Array.last":             {copy: copySome},
	"alloc.Box.get":                {copy: copyValue},
	"array.Array.tryFilter":        {copy: copyValue},
	"mem.UnsafeCell.get":           {copy: copyValue},
	"mem.copyInitialized":          {copy: copyValue},
	"error.as":                     {copy: copySome},
	"fixed.fill":                   {copy: copyElements},
}

// directTry holds the ops whose registry entry has a direct result form: a call site that takes the payload bare skips the wrap.
var directTry = func() map[string]bool {
	out := map[string]bool{}
	for _, op := range mir.Registry.Ops {
		if op.Direct == "TryRef" || op.Direct == "TryOpt" {
			for _, k := range op.Keys {
				out[k] = true
			}
		}
	}
	return out
}()

// planRefused takes the allocations a planned op makes before its result exists.
func (m *Machine) planRefused(key string) bool { return m.ticks(tryPlans[key].own) }

// okPlanned wraps the result of a planned op.
func (m *Machine) okPlanned(key string, v *obj) *obj { return m.okTry(key, tryPlans[key].layers, v) }

// okTry wraps v in Ok as native's try_ok does: layers of variant, each a refusable header and payload, none at a bare call site of a direct op.
func (m *Machine) okTry(key string, layers int, v *obj) *obj {
	refused := false
	if directTry[key] {
		refused = m.bareWrapRefused(layers)
	} else {
		refused = m.wrapRefused(layers)
	}
	if refused {
		m.release(v)
		return m.allocError()
	}
	return m.ok(v)
}
