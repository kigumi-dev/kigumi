package llgen

import (
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/compiler/mir/instance"
	"kigumi/internal/compiler/sem"
)

// ttNest mirrors TT_NEST in rt_types.inc: argument code ttNest - j names template j.
const ttNest = -(1 << 24)

// tmplTable lists the shapes of the runtime types the module composes, each with its parameters numbered as
// holes, so every site that composes one shape shares its template.
type tmplTable struct {
	index map[string]int
	defs  []string
	args  []string
}

// compose emits the node of runtime type t, whose parameters holes carry.
func (e *emitter) compose(sb *strings.Builder, t sem.TypeID, holes map[sem.EntityID]tnHole) string {
	r := e.r
	if instance.Concrete(r, t) {
		return e.tnLoad(sb, t)
	}
	params := instance.Params(r, t)
	vals := make([]string, len(params))
	for i, p := range params {
		h, ok := holes[p]
		switch {
		case !ok:
			e.irb(sb).Call(rtPanic, vptr(e.cstr("internal error: a runtime type names parameters with no environment")))
			return "null"
		case h.value:
			ty := r.TypeParam(p).ConstType
			vals[i] = e.irb(sb).Call(rtTnConst, vptr(h.val), vi64(int(e.constCtor(ty))), vi32(e.constNk(ty))).Text
		default:
			vals[i] = h.val
		}
	}
	if n := r.Types.Node(t); n.Kind == sem.KParam {
		return vals[0]
	}
	order := make(map[sem.EntityID]int, len(params))
	for i, p := range params {
		order[p] = i
	}
	k := vi32(e.tmplIndex(t, order))
	if len(vals) == 1 {
		return e.irb(sb).Call(rtTnTmpl1, k, vptr(vals[0])).Text
	}
	return e.irb(sb).Call(rtTnTmpl, k, vptr(e.argArray(sb, vals))).Text
}

// tmplIndex is the template of non-concrete runtime type t, whose parameters order numbers.
func (e *emitter) tmplIndex(t sem.TypeID, order map[sem.EntityID]int) int {
	tt := &e.tn.tmpl
	args := e.tnArgs(t)
	codes := make([]string, len(args))
	for i, a := range args {
		codes[i] = strconv.Itoa(e.tmplArg(a, order))
	}
	def := fmt.Sprintf("%s { i64 %d, ptr %s, ptr null, i32 %%d, i32 %d }", tndefType, e.ctorID(t), e.tnBase(t), len(args))
	key := def + "/" + strings.Join(codes, ",")
	if k, ok := tt.index[key]; ok {
		return k
	}
	if tt.index == nil {
		tt.index = map[string]int{}
	}
	k := len(tt.defs)
	tt.defs = append(tt.defs, fmt.Sprintf(def, len(tt.args)))
	for _, c := range codes {
		tt.args = append(tt.args, "i32 "+c)
	}
	tt.index[key] = k
	return k
}

// tmplArg codes argument a of a template: a static node, a hole, or a nested template.
func (e *emitter) tmplArg(a sem.TypeID, order map[sem.EntityID]int) int {
	r := e.r
	switch {
	case instance.Concrete(r, a):
		return e.tnIndex(a)
	case r.Types.Kind(a) == sem.KParam:
		return -1 - order[r.Types.Node(a).Ent]
	}
	return ttNest - e.tmplIndex(a, order)
}
