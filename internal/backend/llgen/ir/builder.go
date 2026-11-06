package ir

import (
	"fmt"
	"strings"
)

// Builder emits typed LLVM IR into one function body, panicking on an
// arity or type mismatch instead of emitting bad IR. Temporary names come
// from next; note, when non-nil, observes every runtime call.
type Builder struct {
	sb   *strings.Builder
	next func() string
	note func(RuntimeFn)
}

// NewBuilder returns a Builder over sb. next must be non-nil; note may be nil.
func NewBuilder(sb *strings.Builder, next func() string, note func(RuntimeFn)) Builder {
	if next == nil {
		panic("irb: Builder needs a temporary counter")
	}
	return Builder{sb: sb, next: next, note: note}
}

// Call emits a call to fn and returns its result (TVoid, text "" for a
// void callee).
func (b Builder) Call(fn RuntimeFn, args ...Value) Value {
	if fn.Ret == TVoid {
		b.emit("", fn, args)
		return Value{Type: TVoid}
	}
	t := b.next()
	b.emit(t, fn, args)
	return Value{Type: fn.Ret, Text: t}
}

// CallAs is Call for a caller that pre-assigns its own SSA register name
// instead of taking one from the shared counter.
func (b Builder) CallAs(dst string, fn RuntimeFn, args ...Value) { b.emit(dst, fn, args) }

func (b Builder) emit(dst string, fn RuntimeFn, args []Value) {
	if b.note != nil {
		b.note(fn)
	}
	if len(args) != len(fn.Params) {
		panic(fmt.Sprintf("irb: %s takes %d argument(s), got %d", fn.Name, len(fn.Params), len(args)))
	}
	parts := make([]string, len(args))
	for i, a := range args {
		if a.Type != fn.Params[i] {
			panic(fmt.Sprintf("irb: %s argument %d: want %s, got %s", fn.Name, i, fn.Params[i], a.Type))
		}
		parts[i] = a.Render()
	}
	if dst == "" {
		fmt.Fprintf(b.sb, "  call %s @%s(%s)\n", fn.Ret, fn.Name, strings.Join(parts, ", "))
		return
	}
	fmt.Fprintf(b.sb, "  %s = call %s @%s(%s)\n", dst, fn.Ret, fn.Name, strings.Join(parts, ", "))
}

// GlobalStruct defines a `global` of struct type st and returns name unchanged.
func (b Builder) GlobalStruct(name string, st StructType, fields ...Value) string {
	if len(fields) != len(st.Fields) {
		panic(fmt.Sprintf("irb: %s takes %d field(s), got %d", st.Name, len(st.Fields), len(fields)))
	}
	parts := make([]string, len(fields))
	for i, f := range fields {
		if f.Type != st.Fields[i] {
			panic(fmt.Sprintf("irb: %s field %d: want %s, got %s", st.Name, i, st.Fields[i], f.Type))
		}
		parts[i] = f.Render()
	}
	fmt.Fprintf(b.sb, "%s = global %s { %s }\n", name, st.Name, strings.Join(parts, ", "))
	return name
}
