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

// shareableVariant reports whether one value may stand for every
// construction of ent: nothing may rewrite it through its own storage.
func (e *emitter) shareableVariant(ent sem.EntityID) bool {
	return !e.replacedWhole(e.r.Entity(ent).Parent) && !e.rewritten[ent]
}

// definesDst reports whether op writes its Dst local; the exceptions match
// mir's own printer.
func definesDst(op mir.Opcode) bool {
	switch op {
	case mir.OpDrop, mir.OpSetField, mir.OpSetIndex, mir.OpCellSet,
		mir.OpBoxReplace, mir.OpGlobalSet, mir.OpPanic, mir.OpNop, mir.OpRelease:
		return false
	}
	return true
}

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
	out := map[mir.LocalID]string{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !definesDst(in.Op) || defs[in.Dst] != 1 || f.Locals[in.Dst].Cell {
				continue
			}
			switch in.Op {
			case mir.OpUnit:
				e.registerConst(constUnitKey, constVal{})
				out[in.Dst] = constUnitKey
			case mir.OpConst:
				if k, ok := e.litKey(in); ok {
					out[in.Dst] = k
				}
			case mir.OpAlias, mir.OpMove, mir.OpCopy, mir.OpShare:
				if k, ok := out[in.Args[0]]; ok {
					out[in.Dst] = k
				}
			case mir.OpVariant:
				if k, ok := e.constVariantKey(in, out); ok {
					out[in.Dst] = k
				}
			}
			// Unit carries nothing and rt_unit() is its only object, so any
			// one stands for the rest; the site releases what it was handed.
			if _, keyed := out[in.Dst]; !keyed && f.Locals[in.Dst].Type == sem.TyUnit {
				e.registerConst(constUnitKey, constVal{})
				out[in.Dst], e.constOwned[in.Dst] = constUnitKey, true
			}
		}
	}
	return out
}

// constVariantKey keys in's value when every payload of it is itself
// fixed, recording the shape kg_init_immortals builds it from.
func (e *emitter) constVariantKey(in mir.Inst, keys map[mir.LocalID]string) (string, bool) {
	if !e.shareableVariant(in.Ent) {
		return "", false
	}
	d := e.vdesc(in.Ent)
	if len(in.Args) == 0 {
		e.registerConst(d, constVal{vdesc: d})
		return d, true
	}
	args := make([]string, 0, len(in.Args))
	for _, a := range in.Args {
		k, ok := keys[a]
		if !ok {
			return "", false
		}
		args = append(args, k)
	}
	key := d + "(" + strings.Join(args, ",") + ")"
	e.registerConst(key, constVal{vdesc: d, args: args})
	return key, true
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
