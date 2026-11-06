package llgen

import (
	"strconv"
	"strings"

	"kigumi/internal/backend/llgen/ir"
)

// The pure IR-building layer lives in llgen/ir; these aliases keep the
// emitter's call sites unchanged.
type (
	Type       = ir.Type
	Value      = ir.Value
	RuntimeFn  = ir.RuntimeFn
	StructType = ir.StructType
	Builder    = ir.Builder
)

const (
	TVoid    = ir.TVoid
	TPtr     = ir.TPtr
	TI1      = ir.TI1
	TI32     = ir.TI32
	TI64     = ir.TI64
	TDouble  = ir.TDouble
	TObjSlot = ir.TObjSlot
	TI8      = ir.TI8
	TI16     = ir.TI16
	TI128    = ir.TI128
	TFloat   = ir.TFloat
	TCPtr    = ir.TCPtr
)

// irb returns a Builder over sb, sharing the emitter's temporary counter.
func (e *emitter) irb(sb *strings.Builder) Builder {
	var note func(RuntimeFn)
	if e.boxes != nil {
		note = e.noteRuntimeCall
	}
	return ir.NewBuilder(sb, e.newTmp, note)
}

func vptr(text string) Value     { return Value{Type: TPtr, Text: text} }
func vi32(n int) Value           { return Value{Type: TI32, Text: strconv.Itoa(n)} }
func vi32text(text string) Value { return Value{Type: TI32, Text: text} }
func vi64(n int) Value           { return Value{Type: TI64, Text: strconv.Itoa(n)} }
func vi64text(text string) Value { return Value{Type: TI64, Text: text} }
func vdouble(text string) Value  { return Value{Type: TDouble, Text: text} }
func vobjslot() Value            { return Value{Type: TObjSlot, Text: "zeroinitializer"} }
