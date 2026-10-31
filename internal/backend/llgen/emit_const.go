package llgen

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func (e *emitter) constant(sb *strings.Builder, in mir.Inst) string {
	if key, ok := e.litKey(in); ok {
		return e.constSlotName(key)
	}
	switch in.Lit.Kind {
	case sem.LitBool:
		b := "0"
		if in.Lit.Bool {
			b = "1"
		}
		return e.boolBox(sb, b)
	case sem.LitRecord:
		return e.recordConstant(sb, in)
	case sem.LitBytes:
		// secureErase rewrites a Bytes in place, so a literal cannot be one shared object.
		return e.irb(sb).Call(rtBytes, vptr(e.cstr(in.Lit.Str)), vi64(len(in.Lit.Str))).Text
	}
	return e.irb(sb).Call(rtUnit).Text
}

func (e *emitter) recordConstant(sb *strings.Builder, in mir.Inst) string {
	ent := e.r.Types.Node(in.Type).Ent
	fields := e.r.TypeDecl(ent).Fields
	vals := make([]string, len(fields))
	for i, f := range fields {
		vals[i] = e.constant(sb, mir.Inst{Lit: in.Lit.Fields[i], Type: e.r.Entity(f).Type})
	}
	arr := e.argArray(sb, vals)
	return e.irb(sb).Call(rtRecord, vptr(e.desc(ent)), vi64(len(vals)), vptr(arr)).Text
}

// litKey registers in's literal as an immortal and names its slot key;
// Bool and Void have one static object each already, so they stay out.
func (e *emitter) litKey(in mir.Inst) (string, bool) {
	lit := in.Lit
	tt := e.r.Types
	switch {
	case in.Str != "" && tt.IsInteger(in.Type):
		return e.constNum(kInt, e.numKind(in.Type), in.Str), true
	case tt.IsFloat(in.Type):
		f := 0.0
		if lit.Float != nil {
			f, _ = lit.Float.Float64()
		} else if lit.Int != nil {
			f, _ = new(big.Float).SetInt(lit.Int).Float64()
		}
		return e.constNum(kFloat, e.numKind(in.Type), fmt.Sprintf("0x%X", math.Float64bits(f))), true
	case lit.Kind == sem.LitInt && lit.Int != nil && tt.IsInteger(e.r.UnderlyingRepr(in.Type)) && tt.Width(e.r.UnderlyingRepr(in.Type)) == 128:
		return e.constNum(kInt, e.numKind(in.Type), lit.Int.String()), true
	case lit.Kind == sem.LitInt:
		v := "0"
		if lit.Int != nil {
			if lit.Int.IsInt64() {
				v = lit.Int.String()
			} else {
				v = fmt.Sprintf("%d", int64(lit.Int.Uint64()))
			}
		}
		return e.constNum(kInt, e.numKind(in.Type), v), true
	case lit.Kind == sem.LitChar:
		return e.constNum(kChar, 0, strconv.Itoa(int(lit.Char))), true
	case lit.Kind == sem.LitString:
		return e.constText(kStr, lit.Str), true
	}
	return "", false
}

func (e *emitter) interp(sb *strings.Builder, in mir.Inst, args []string) string {
	var items []string
	ai := 0
	for _, s := range in.Strs {
		if s == "" {
			items = append(items, args[ai])
			ai++
			continue
		}
		items = append(items, e.constSlotName(e.constText(kStr, s)))
	}
	arr := e.argArray(sb, items)
	return e.irb(sb).Call(rtInterp, vi32(len(items)), vptr(arr)).Text
}

// shell turns a `$"..."` literal into a std/shell.Plan, one
// std/shell.Item builder call per piece, in source order.
func (e *emitter) shell(sb *strings.Builder, in mir.Inst, args []string) string {
	plan := e.callStd(sb, "std/shell", "planNew", nil)
	ai := 0
	push := func(item string) { plan = e.callStd(sb, "std/shell", "planPush", []string{plan, item}) }
	for _, s := range in.Strs {
		switch {
		case s == "|":
			push(e.shellItem(sb, "Pipe", nil))
		case s == " ":
			push(e.shellItem(sb, "Word", nil))
		case s == "$":
			text := e.irb(sb).Call(rtDisplay, vptr(args[ai])).Text
			ai++
			push(e.shellItem(sb, "Text", []string{text}))
		case s[0] == 'r':
			op, fd, _ := strings.Cut(s[1:], ":")
			opv := e.constSlotName(e.constNum(kInt, 320, op))
			fdv := e.constSlotName(e.constNum(kInt, 320, fd))
			push(e.shellItem(sb, "Redirect", []string{opv, fdv}))
		default:
			text := s[1:]
			push(e.shellItem(sb, "Text", []string{e.constSlotName(e.constText(kStr, text))}))
		}
	}
	return plan
}

func (e *emitter) shellItem(sb *strings.Builder, name string, payload []string) string {
	item := e.r.PackageMember(e.r.PackageByPath("std/shell"), "Item")
	for _, v := range e.r.TypeDecl(item).Variants {
		if e.r.Entity(v).Name == name {
			arr := e.argArray(sb, payload)
			return e.irb(sb).Call(rtVariant, vptr(e.vdesc(v)), vi64(len(payload)), vptr(arr)).Text
		}
	}
	panic("std/shell.Item has no variant " + name)
}

func (e *emitter) callStd(sb *strings.Builder, pkg, name string, args []string) string {
	fn := e.p.ByEnt[e.r.PackageMember(e.r.PackageByPath(pkg), name)]
	return e.callBoxed(sb, fn, args)
}
