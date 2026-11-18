package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir/instance"
	"kigumi/internal/compiler/sem"
)

// tndefType mirrors rt_tndef in rt_types.inc.
const tndefType = "{ i64, ptr, ptr, i32, i32 }"

// tnTable lists the runtime types the module names statically, interned at startup into @rt_tn in order, each
// after its arguments.
type tnTable struct {
	index map[sem.TypeID]int
	defs  []string
	args  []string
	tmpl  tmplTable
}

// canon is t's runtime identity.
func (e *emitter) canon(t sem.TypeID) sem.TypeID { return e.p.Canon.Of(e.r, t) }

// tnArgs are the argument types of runtime type t, its element last.
func (e *emitter) tnArgs(t sem.TypeID) []sem.TypeID {
	n := e.r.Types.Node(t)
	out := append([]sem.TypeID(nil), n.Args...)
	if n.Elem != 0 && n.Kind != sem.KConst {
		out = append(out, n.Elem)
	}
	return out
}

// tnBase is the descriptor a box or record of instantiation t is made from: its generic type's.
func (e *emitter) tnBase(t sem.TypeID) string {
	n := e.r.Types.Node(t)
	if n.Kind != sem.KNamed || e.r.Entity(n.Ent).Kind != sem.EntType || len(e.r.TypeDecl(n.Ent).Params) == 0 || !e.keepsType(n.Ent) {
		return "null"
	}
	return e.desc(n.Ent)
}

func (e *emitter) keepsType(ent sem.EntityID) bool {
	return e.reachable == nil || e.reachable.KeepType(ent)
}

// tnStaticDesc is the descriptor a static node of t carries from the start: a non-generic type's own, so its boxes
// answer as that node.
func (e *emitter) tnStaticDesc(t sem.TypeID) string {
	n := e.r.Types.Node(t)
	switch n.Kind {
	case sem.KNamed:
		if e.r.Entity(n.Ent).Kind != sem.EntType || !e.keepsType(n.Ent) {
			return "null"
		}
		if len(e.r.TypeDecl(n.Ent).Params) == 0 {
			return e.desc(n.Ent)
		}
		return e.simdInstDesc(t)
	case sem.KPrim:
		if ent := e.r.EntityOfName(e.r.TypeString(t)); ent != 0 && !e.keepsType(ent) {
			return "null"
		}
		return e.descOfType(t)
	}
	return "null"
}

// tnIndex is the place of concrete runtime type t in the static table.
func (e *emitter) tnIndex(t sem.TypeID) int {
	t = e.canon(t)
	if k, ok := e.tn.index[t]; ok {
		return k
	}
	if e.tn.index == nil {
		e.tn.index = map[sem.TypeID]int{}
	}
	var def string
	if e.r.Types.Kind(t) == sem.KConst {
		v, _ := e.r.Types.ConstValueBig(t)
		lo, hi := instance.ConstWords(v)
		def = fmt.Sprintf("%s { i64 %d, ptr null, ptr null, i32 %d, i32 -1 }", tndefType, e.ctorID(t), len(e.tn.args))
		for _, w := range []uint64{lo, hi} {
			e.tn.args = append(e.tn.args, fmt.Sprintf("i32 %d", int32(uint32(w))), fmt.Sprintf("i32 %d", int32(uint32(w>>32))))
		}
	} else {
		args := e.tnArgs(t)
		ks := make([]int, len(args))
		for i, a := range args {
			ks[i] = e.tnIndex(a)
		}
		def = fmt.Sprintf("%s { i64 %d, ptr %s, ptr %s, i32 %d, i32 %d }", tndefType, e.ctorID(t), e.tnBase(t), e.tnStaticDesc(t), len(e.tn.args), len(ks))
		for _, k := range ks {
			e.tn.args = append(e.tn.args, fmt.Sprintf("i32 %d", k))
		}
	}
	k := len(e.tn.defs)
	e.tn.defs = append(e.tn.defs, def)
	e.tn.index[t] = k
	return k
}

// tnLoad loads the node of concrete runtime type t.
func (e *emitter) tnLoad(sb *strings.Builder, t sem.TypeID) string {
	k := e.tnIndex(t)
	slot := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr ptr, ptr @rt_tn, i64 %d\n", slot, k)
	v := e.newTmp()
	fmt.Fprintf(sb, "  %s = load ptr, ptr %s\n", v, slot)
	return v
}

// tnHole is the runtime value a type.of fills a parameter with: a runtime type, or an opened const's value.
type tnHole struct {
	val   string
	value bool
}

// constNk is the number kind a const of type ty reads back as, 0 for a Bool.
func (e *emitter) constNk(ty sem.TypeID) int {
	if ty == sem.TyBool {
		return 0
	}
	return e.numKind(ty)
}

// initTn interns the static runtime types before any static initializer runs.
func (e *emitter) initTn(sb *strings.Builder) {
	if len(e.tn.defs) == 0 && len(e.tn.tmpl.defs) == 0 {
		return
	}
	e.irb(sb).Call(rtTnInit, vi64(len(e.tn.defs)), vptr("@rt_tn_defs"), vptr("@rt_tn_args"), vptr("@rt_tn"), vi64(len(e.tn.tmpl.defs)), vptr("@rt_tt"), vptr("@rt_tt_args"))
}

func (e *emitter) tnGlobals() string {
	if len(e.tn.defs) == 0 && len(e.tn.tmpl.defs) == 0 {
		return ""
	}
	var sb strings.Builder
	tndefs(&sb, "@rt_tn_defs", "@rt_tn_args", e.tn.defs, e.tn.args)
	tndefs(&sb, "@rt_tt", "@rt_tt_args", e.tn.tmpl.defs, e.tn.tmpl.args)
	fmt.Fprintf(&sb, "@rt_tn = internal global [%d x ptr] zeroinitializer\n", max(len(e.tn.defs), 1))
	return sb.String()
}

func tndefs(sb *strings.Builder, defsName, argsName string, defs, args []string) {
	if len(defs) == 0 {
		defs = []string{tndefType + " zeroinitializer"}
	}
	if len(args) == 0 {
		args = []string{"i32 0"}
	}
	fmt.Fprintf(sb, "%s = private constant [%d x %s] [%s]\n", defsName, len(defs), tndefType, strings.Join(defs, ", "))
	fmt.Fprintf(sb, "%s = private constant [%d x i32] [%s]\n", argsName, len(args), strings.Join(args, ", "))
}
