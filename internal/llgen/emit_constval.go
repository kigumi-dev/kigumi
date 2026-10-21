package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// A key without parentheses names a payload-free variant by its descriptor;
// one with them, a variant whose payload values are all themselves fixed.
const constUnitKey = "unit"

// constVal is a value fixed for the whole program, so every site that
// builds it can share one immortal instead of allocating.
type constVal struct {
	vdesc string
	args  []string
	lit   *constLit
}

// needsSlot reports whether cv is built in reserved storage; a payload-free
// variant lives in its own descriptor instead.
func (cv constVal) needsSlot() bool { return cv.lit != nil || len(cv.args) > 0 }

func (e *emitter) constSlotName(key string) string {
	return fmt.Sprintf("@\".constval.%d\"", e.constIndex[key])
}

func (e *emitter) constPayloadName(key string) string {
	return fmt.Sprintf("@\".constval.%d.payload\"", e.constIndex[key])
}

func definesDst(op mir.Opcode) bool { return op.Yields() }

// constLocals keys the locals of f holding a value fixed at compile time; a
// local written more than once is left out, since a key has to name one value.
func (e *emitter) constLocals(f *mir.Func) map[mir.LocalID]string {
	defs := map[mir.LocalID]int{}
	for _, p := range f.Params {
		defs[p]++
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if definesDst(in.Op) {
				defs[in.Dst]++
			}
		}
	}
	out, shape := map[mir.LocalID]string{}, map[mir.LocalID]string{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !definesDst(in.Op) || defs[in.Dst] != 1 || f.Locals[in.Dst].Cell {
				continue
			}
			switch in.Op {
			case mir.OpUnit:
				e.registerConst(constUnitKey, constVal{})
				out[in.Dst], shape[in.Dst] = constUnitKey, constUnitKey
			case mir.OpConst:
				if k, ok := e.litKey(in); ok {
					out[in.Dst], shape[in.Dst] = k, k
				}
			case mir.OpAlias, mir.OpMove, mir.OpCopy, mir.OpShare:
				if k, ok := shape[in.Args[0]]; ok {
					shape[in.Dst] = k
					if _, boxed := out[in.Args[0]]; boxed {
						out[in.Dst] = k
					}
				}
			case mir.OpVariant:
				if k, ok := e.variantShape(in, shape); ok {
					shape[in.Dst] = k
					if !e.isDirect(in.Dst) {
						e.commitConst(k)
						out[in.Dst] = k
					}
				}
			case mir.OpBoxDirect:
				// The VM counts no allocation where mir marks a crossing fixed, so mir alone decides.
				if in.Fixed {
					k := fixedShape(shape, in.Args[0])
					e.commitConst(k)
					out[in.Dst], shape[in.Dst] = k, k
				}
			}
			// Void carries nothing and rt_unit() is its only object, so any
			// one stands for the rest; the site releases what it was handed.
			if _, keyed := out[in.Dst]; !keyed && f.Locals[in.Dst].Type == sem.TyUnit {
				e.registerConst(constUnitKey, constVal{})
				out[in.Dst], shape[in.Dst], e.constOwned[in.Dst] = constUnitKey, constUnitKey, true
			}
		}
	}
	return out
}

func (e *emitter) registerConst(key string, cv constVal) {
	if _, seen := e.constVals[key]; seen {
		return
	}
	e.constVals[key] = cv
	e.constIndex[key] = len(e.constOrder)
	e.constOrder = append(e.constOrder, key)
}

// constSlots defines the storage each shared variant is built in; the
// payload array lives beside it, since the value points into it.
func (e *emitter) constSlots() string {
	var sb strings.Builder
	for _, key := range e.constOrder {
		cv := e.constVals[key]
		if !cv.needsSlot() {
			continue
		}
		fmt.Fprintf(&sb, "%s = internal global %s zeroinitializer, align 16\n", e.constSlotName(key), objSlotType())
		if len(cv.args) > 0 {
			fmt.Fprintf(&sb, "%s = internal global [%d x ptr] zeroinitializer, align 8\n", e.constPayloadName(key), len(cv.args))
		}
	}
	return sb.String()
}

// initConstVals fills every shared variant before any Kigumi code runs,
// for the reason kg_init_immortals exists at all.
func (e *emitter) initConstVals(sb *strings.Builder) {
	for _, key := range e.constOrder {
		cv := e.constVals[key]
		if cv.lit != nil {
			e.initLit(sb, e.constSlotName(key), cv.lit)
			continue
		}
		if len(cv.args) == 0 {
			continue
		}
		payload := e.constPayloadName(key)
		for i, ak := range cv.args {
			v := e.constArgValue(sb, ak)
			p := e.newTmp()
			fmt.Fprintf(sb, "  %s = getelementptr ptr, ptr %s, i64 %d\n", p, payload, i)
			fmt.Fprintf(sb, "  store ptr %s, ptr %s\n", v, p)
		}
		e.irb(sb).Call(rtConstVariant, vptr(e.constSlotName(key)), vptr(payload), vptr(cv.vdesc), vi64(len(cv.args)))
	}
}

func (e *emitter) constArgValue(sb *strings.Builder, key string) string {
	cv := e.constVals[key]
	switch {
	case key == constUnitKey:
		return e.irb(sb).Call(rtUnit).Text
	case cv.lit == nil && len(cv.args) == 0:
		return e.irb(sb).Call(rtNullary, vptr(cv.vdesc)).Text
	}
	return e.constSlotName(key)
}
