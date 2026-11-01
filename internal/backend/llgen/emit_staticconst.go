package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// constRecordKey names the constval slot that holds an ImmortalStatic's value.
func (e *emitter) constRecordKey(ent sem.EntityID, lit sem.Literal) (string, bool) {
	if !e.r.ImmortalStatic(ent) {
		return "", false
	}
	return e.recordKey(lit, e.r.Entity(ent).Type), true
}

func (e *emitter) recordKey(lit sem.Literal, typ sem.TypeID) string {
	ent := e.r.Types.Node(typ).Ent
	fields := e.r.TypeDecl(ent).Fields
	keys := make([]string, len(fields))
	for i, f := range fields {
		ft, fl := e.r.Entity(f).Type, lit.Fields[i]
		switch {
		case fl.Kind == sem.LitRecord:
			keys[i] = e.recordKey(fl, ft)
		case fl.Kind == sem.LitBool && fl.Bool:
			keys[i] = constTrueKey
		case fl.Kind == sem.LitBool:
			keys[i] = constFalseKey
		default:
			keys[i], _ = e.litKey(midir.Inst{Lit: fl, Type: ft})
		}
	}
	key := "rec:" + e.desc(ent) + "(" + strings.Join(keys, ",") + ")"
	e.registerConst(key, constVal{rec: e.desc(ent), args: keys})
	return key
}
