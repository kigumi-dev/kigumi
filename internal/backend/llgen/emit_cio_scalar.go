package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/sem"
)

// convertCall passes only the target kind to rt_convert; the source kind
// travels with the value itself.
func (e *emitter) convertCall(sb *strings.Builder, ent sem.EntityID, key string, args []string, self, dst sem.TypeID) (string, bool) {
	tt := e.r.Types
	if !strings.HasPrefix(key, "std/prelude.") || !e.r.IsConversion(ent) {
		return "", false
	}
	if n := tt.Node(self); n.Kind == sem.KRef {
		self = n.Elem
	}
	if !tt.IsNumeric(self) && self != sem.TyChar {
		return "", false
	}
	target, checked := dst, 0
	if n := tt.Node(dst); n.Kind == sem.KNamed && n.Ent == tt.OptionEnt() {
		target, checked = n.Args[0], 1
	}
	dnk := rtConvertCharTarget
	if target != sem.TyChar {
		if !tt.IsNumeric(target) {
			return "", false
		}
		dnk = e.numKind(target)
	}
	return e.irb(sb).Call(rtConvert, vptr(args[0]), vi32(dnk), vi32(checked)).Text, true
}

// loweredStd reports whether a bodiless std function is expanded by the
// compiler rather than rt_std, so its function value needs that same adapter.
func (e *emitter) loweredStd(fn sem.EntityID) bool {
	key := e.stdKey(fn)
	if strings.HasPrefix(key, "std/ffi.") || strings.HasPrefix(key, "std/mem.volatile") || key == "std/error.as" {
		return true
	}
	info := e.r.Fn(fn)
	if !strings.HasPrefix(key, "std/prelude.") || !e.r.IsConversion(fn) {
		return false
	}
	self := e.r.Entity(info.Owner).Type
	return e.r.Types.IsNumeric(self) || self == sem.TyChar
}

// stdBound has one adapter per value type: a generic primitive expands per instantiation.
func (e *emitter) stdBound(fn sem.EntityID, bound int, value sem.TypeID) string {
	sym := fmt.Sprintf("@\"%s$stdbound%d$%s\"", e.stdKey(fn), bound, e.r.TypeKey(value))
	if !e.cio[sym] {
		e.cio[sym] = true
		e.helpers.WriteString(e.stdAdapterWith(fn, sym, bound, value))
	}
	return sym
}
