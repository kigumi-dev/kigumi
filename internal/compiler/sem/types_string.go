package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

var primNames = map[TypeID]string{
	TyPoison: "<error>", TyNever: "Never", TyUnit: "Void", TyBool: "Bool", TyChar: "Char",
	TyString: "String", TyBytes: "Bytes", TyI8: "i8", TyI16: "i16", TyI32: "i32", TyI64: "Int",
	TyI128: "i128", TyU8: "u8", TyU16: "u16", TyU32: "u32", TyU64: "u64", TyU128: "u128",
	TyUsize: "usize", TyIsize: "isize", TyF32: "f32", TyF64: "Float",
	TyUntypedInt: "{integer}", TyUntypedFloat: "{float}",
}

const (
	// typeRenderBudget bounds one spelled-out type. The type table is a DAG, and spelling every path of a shared
	// node expands it to a tree: a depth-20 nested pair has 2^20 paths, so an unbounded spelling prints megabytes
	// (and spends exponential time doing it). A clipped spelling keeps a diagnostic short and readable.
	typeRenderBudget = 2048
	// typeRenderDepth bounds how deep a spelling nests before it elides the rest. Without it the budget is spent
	// on one branch of a shared type, which reads worse than `((…, …), …)`.
	typeRenderDepth = 8
	// typeRenderElision marks a clipped type in a diagnostic.
	typeRenderElision = "…"
)

// typeWriter spells a type into a bounded builder: writes past the budget are dropped and a nesting limit elides
// what it cannot spell, so str reports the clip with a trailing ellipsis.
type typeWriter struct {
	sb        strings.Builder
	left      int
	depth     int
	truncated bool
}

func newTypeWriter() *typeWriter { return &typeWriter{left: typeRenderBudget} }

// write appends s unless it does not fit, in which case the budget is spent and the text is dropped whole: a
// half-written identifier would be misleading.
func (w *typeWriter) write(s string) {
	if len(s) > w.left {
		w.left = 0
		w.truncated = true
		return
	}
	w.sb.WriteString(s)
	w.left -= len(s)
}

// spent reports that the budget is gone, so no further token can be spelled.
func (w *typeWriter) spent() bool { return w.left <= 0 }

func (w *typeWriter) str() string {
	if w.truncated {
		return w.sb.String() + typeRenderElision
	}
	return w.sb.String()
}

// TypeString renders a type in source notation: `T?`, `T!`, `Int` for i64,
// `Float` for f64, `pure fn(A) -> R`. The spelling is clipped once it reaches
// typeRenderBudget bytes or typeRenderDepth nesting levels, ending in `…`.
func (r *Result) TypeString(t TypeID) string {
	w := newTypeWriter()
	r.writeType(w, t)
	return w.str()
}

func (r *Result) writeType(w *typeWriter, t TypeID) {
	if w.spent() {
		w.truncated = true
		return
	}
	if w.depth >= typeRenderDepth {
		w.write(typeRenderElision)
		return
	}
	w.depth++
	r.writeTypeNode(w, t)
	w.depth--
}

func (r *Result) writeTypeNode(w *typeWriter, t TypeID) {
	if t == 0 {
		w.write("<none>")
		return
	}
	if name, ok := primNames[t]; ok {
		w.write(name)
		return
	}
	tt := r.Types
	n := tt.nodes[t]
	switch n.Kind {
	case KNamed, KIface:
		if elem, ok := tt.IsOption(t); ok {
			r.writeType(w, elem)
			w.write("?")
			return
		}
		if val, err, ok := tt.IsResult(t); ok && err == tt.errorType() {
			r.writeType(w, val)
			w.write("!")
			return
		}
		if tt.isTupleEnt(n.Ent) {
			w.write("(")
			for i, a := range n.Args {
				if i > 0 {
					w.write(", ")
				}
				r.writeType(w, a)
			}
			w.write(")")
			return
		}
		w.write(r.entityName(n.Ent))
		r.writeArgs(w, n.Args)
	case KParam:
		w.write(r.entityName(n.Ent))
	case KConst:
		v, ty := tt.ConstValue(t)
		if ty == TyBool {
			w.write(map[int64]string{0: "false", 1: "true"}[v])
			return
		}
		w.write(itoa64(v))
	case KVar:
		if i := int(n.Var); i < len(r.varNames) && r.varNames[i] != "" {
			w.write(r.varNames[i])
			return
		}
		w.write("_")
	case KFn:
		r.writeFn(w, n)
	case KClosure:
		w.write("closure ")
		if id := n.Ent; id != 0 {
			r.writeFn(w, tt.nodes[r.closure(id).Sig])
		}
	case KRef:
		w.write("&")
		if n.Lifetime != 0 && tt.nodes[n.Lifetime].Kind == KParam {
			r.writeType(w, n.Lifetime)
			w.write(" ")
		}
		if n.Flags&flagMut != 0 {
			w.write("mut ")
		}
		r.writeType(w, n.Elem)
	case KPtr:
		if n.Flags&flagMut != 0 {
			w.write("*mut ")
		} else {
			w.write("*const ")
		}
		r.writeType(w, n.Elem)
	default:
		w.write("<?>")
	}
}

func (r *Result) writeArgs(w *typeWriter, args []TypeID) {
	if len(args) == 0 {
		return
	}
	w.write("[")
	for i, a := range args {
		if i > 0 {
			w.write(", ")
		}
		r.writeType(w, a)
	}
	w.write("]")
}

func (tt *TypeTable) errorType() TypeID {
	if tt.errorEnt == 0 {
		return 0
	}
	return tt.Iface(tt.errorEnt, nil)
}

// TypeStringAt renders the type of node n, keeping the alias spelling the
// source used when the node refers to an alias.
func (r *Result) TypeStringAt(t *syntax.Tree, n syntax.NodeID) string {
	f := r.File(t)
	if f == nil || int(n) >= len(f.Types) {
		return ""
	}
	if use := f.Uses[n]; use != 0 && r.Entities[use].Kind == EntAlias {
		return r.Entities[use].Name
	}
	return r.TypeString(f.Types[n])
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func itoa(n int) string { return itoa64(int64(n)) }
