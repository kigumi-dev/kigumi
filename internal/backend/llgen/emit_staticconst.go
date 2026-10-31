package llgen

import (
	"strings"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

// constRecordKey names the immortal for a non-mut static whose value is a record of literals.
func (e *emitter) constRecordKey(ent sem.EntityID, lit sem.Literal) (string, bool) {
	if _, mut := e.r.StaticOwnerOf(ent); mut || !e.fixedRecord(lit, e.r.Entity(ent).Type) {
		return "", false
	}
	return e.recordKey(lit, e.r.Entity(ent).Type), true
}

// A record with a mut field is written in place and rt_copy shares an immortal, so only read-only ones qualify.
func (e *emitter) fixedRecord(lit sem.Literal, typ sem.TypeID) bool {
	ent := e.r.Types.Node(typ).Ent
	if lit.Kind != sem.LitRecord || len(lit.Fields) == 0 || e.r.RecordWritable(ent) {
		return false
	}
	for i, f := range e.r.TypeDecl(ent).Fields {
		switch fl := lit.Fields[i]; fl.Kind {
		case sem.LitRecord:
			if !e.fixedRecord(fl, e.r.Entity(f).Type) {
				return false
			}
		case sem.LitInt, sem.LitFloat, sem.LitString, sem.LitBool, sem.LitChar:
		default:
			return false
		}
	}
	return true
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
			keys[i], _ = e.litKey(mir.Inst{Lit: fl, Type: ft})
		}
	}
	key := "rec:" + e.desc(ent) + "(" + strings.Join(keys, ",") + ")"
	e.registerConst(key, constVal{rec: e.desc(ent), args: keys})
	return key
}
