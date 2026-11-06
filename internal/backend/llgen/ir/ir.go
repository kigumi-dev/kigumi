// Package ir holds the pure LLVM IR text-building layer shared by the
// llgen emitter: typed operands, runtime signatures, and a body builder
// that checks arity and types before printing. Nothing here knows about
// sem, MIR, or emitter state; temporary counters and call notes are
// supplied as callbacks.
package ir

import (
	"fmt"
	"strconv"
)

// Type is an LLVM IR scalar type used by runtime call signatures.
type Type uint8

const (
	TVoid Type = iota
	TPtr
	TI1
	TI32
	TI64
	TDouble
	// TObjSlot is the zeroed storage a descriptor reserves for an immortal
	// value; RT_OBJ_SLOT in rt.h must agree with its size.
	TObjSlot
	TI8
	TI16
	TI128
	TFloat
	// TCPtr is a raw C pointer held in a register, unlike TPtr's runtime value.
	TCPtr
)

// ObjSlotBytes is RT_OBJ_SLOT: the storage one runtime object is built in.
// It must agree with rt.h and internal/compiler/mir/layout.ObjSlotBytes;
// llgen pins all three in TestObjSlotMatchesC.
const ObjSlotBytes = 64

var objSlotType = "[" + strconv.Itoa(ObjSlotBytes) + " x i8]"

func (t Type) String() string {
	switch t {
	case TVoid:
		return "void"
	case TPtr, TCPtr:
		return "ptr"
	case TI1:
		return "i1"
	case TI32:
		return "i32"
	case TI64:
		return "i64"
	case TDouble:
		return "double"
	case TObjSlot:
		return objSlotType
	case TI8:
		return "i8"
	case TI16:
		return "i16"
	case TI128:
		return "i128"
	case TFloat:
		return "float"
	default:
		panic(fmt.Sprintf("irb: unknown type %d", t))
	}
}

// Value is one typed LLVM operand: a register, a global, a constant, or
// "null", paired with the type it is used at.
type Value struct {
	Type Type
	Text string
}

// Render prints the operand in the `type text` form LLVM expects.
func (v Value) Render() string { return v.Type.String() + " " + v.Text }

// RuntimeFn is one C runtime entry point's signature: the single source
// the `declare` list and every call's argument check draw from.
type RuntimeFn struct {
	Name   string
	Params []Type
	Ret    Type
}

// StructType is an LLVM named struct type used by a descriptor global.
type StructType struct {
	Name   string
	Fields []Type
}
