package llgen

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// descAdapter is a method-table slot or drop hook of fn: with inst, for a body specialize made for that one
// instantiation, else reading the environment and the const arguments from the descriptor it is called through.
type descAdapter struct {
	sym  string
	fn   sem.EntityID
	body *midir.Func
	inst sem.TypeID
	// unbound marks the slot of a descriptor reaching a body that only runs specialized, which no
	// specialization was made for: the compiler makes one for every instantiation it boxes.
	unbound bool
	done    bool
}

// useDescAdapter names the adapter a method-table slot or drop hook of fn runs for a value of type t, 0 when the
// slot belongs to the entity's own descriptor.
func (e *emitter) useDescAdapter(fn sem.EntityID, t sem.TypeID) string {
	f := e.p.ByEnt[fn]
	if t != 0 && instance.Concrete(e.r, t) {
		if body := e.p.Desc.Get(e.r, fn, t); body != nil {
			return e.addDescAdapter(&descAdapter{sym: "@\"" + e.symName(body) + "$desc\"", fn: fn, body: body, inst: t})
		}
	}
	if e.p.Desc.Specialized(fn) {
		return e.addDescAdapter(&descAdapter{sym: "@\"" + e.symName(f) + "$desc.unbound\"", fn: fn, body: f, unbound: true})
	}
	info := e.r.Fn(fn)
	if !f.HasEnv && len(info.ConstParams) == 0 && len(info.Witnesses) == 0 {
		return e.useAdapter(f)
	}
	return e.addDescAdapter(&descAdapter{sym: "@\"" + e.symName(f) + "$desc\"", fn: fn, body: f})
}

func (e *emitter) addDescAdapter(req *descAdapter) string {
	if e.descAdapters == nil {
		e.descAdapters = map[string]*descAdapter{}
	}
	if _, ok := e.descAdapters[req.sym]; !ok {
		e.descAdapters[req.sym] = req
	}
	return req.sym
}

func (e *emitter) emitDescAdapters(sb *strings.Builder) bool {
	progress := false
	for _, sym := range slices.Sorted(maps.Keys(e.descAdapters)) {
		req := e.descAdapters[sym]
		if req.done {
			continue
		}
		req.done, progress = true, true
		e.tmp = 0
		sb.WriteString(e.descAdapterFn(req))
	}
	return progress
}

func (e *emitter) descAdapterFn(req *descAdapter) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", req.sym)
	if req.unbound {
		e.irb(&sb).Call(rtPanic, vptr(e.cstr("internal error: `"+e.r.Entity(req.fn).Name+"` has no body for this instantiation")))
		sb.WriteString("  unreachable\n}\n\n")
		return sb.String()
	}
	info := e.r.Fn(req.fn)
	hidden := len(info.Witnesses) + len(info.ConstParams) + boolInt(req.body.HasEnv)
	n := len(req.body.Params) - hidden
	slots := paramSlots(midir.BoundTrails(midir.OpClosure), 0, n)
	var vals []string
	for i := range n {
		v := fmt.Sprintf("%%a%d", i)
		e.slotArg(&sb, v, slots[i])
		vals = append(vals, v)
	}
	if req.body.HasEnv {
		vals = append(vals, e.descEnv(&sb, req.fn))
	}
	for range info.Witnesses {
		vals = append(vals, e.irb(&sb).Call(rtUnit).Text)
	}
	if req.inst != 0 {
		hid, _ := instance.HiddenArgs(e.r, req.fn, req.inst)
		for _, a := range hid.Consts {
			vals = append(vals, e.constant(&sb, instance.ConstInst(a)))
		}
	} else if len(info.ConstParams) > 0 {
		for _, p := range e.r.ConstParamsOf(req.fn) {
			at, _ := e.recvArgIndex(req.fn, p)
			node := e.irb(&sb).Call(rtTnArg, vptr("%env"), vi64(at)).Text
			vals = append(vals, e.irb(&sb).Call(rtTnValue, vptr(node), vi32(e.constNk(e.r.TypeParam(p).ConstType))).Text)
		}
	}
	fmt.Fprintf(&sb, "  ret ptr %s\n}\n\n", e.callBoxed(&sb, req.body, vals))
	return sb.String()
}

// descEnv is the environment method fn runs with when called for a receiver of runtime type %env: that type,
// or fn applied to its arguments.
func (e *emitter) descEnv(sb *strings.Builder, fn sem.EntityID) string {
	env, _ := instance.EnvType(e.r, fn)
	env = e.canon(env)
	n := e.r.Types.Node(env)
	if n.Ent != fn {
		return e.irb(sb).Call(rtTnEnv, vptr("%env"), vi64(0), vi64(-1), vptr("null")).Text
	}
	idx := make([]string, len(n.Args))
	for i, a := range n.Args {
		at, _ := e.recvArgIndex(fn, e.r.Types.Node(a).Ent)
		idx[i] = fmt.Sprintf("i32 %d", at)
	}
	m := fmt.Sprintf("@\"%s$envmap\"", e.symName(e.p.ByEnt[fn]))
	if !e.cio[m] {
		e.cio[m] = true
		fmt.Fprintf(&e.helpers, "%s = private constant [%d x i32] [%s]\n", m, max(len(idx), 1), strings.Join(append(idx, "i32 0")[:max(len(idx), 1)], ", "))
	}
	return e.irb(sb).Call(rtTnEnv, vptr("%env"), vi64(int(e.ctorID(env))), vi64(len(idx)), vptr(m)).Text
}

// recvArgIndex is the position of type parameter p among the arguments of method fn's declared receiver type.
func (e *emitter) recvArgIndex(fn, p sem.EntityID) (int, bool) {
	info := e.r.Fn(fn)
	if info.SelfParam == 0 {
		return 0, false
	}
	self := e.r.Entity(info.SelfParam).Type
	if e.r.Types.Kind(self) == sem.KRef {
		self = e.r.Types.Node(self).Elem
	}
	for i, a := range e.r.Types.Node(self).Args {
		if n := e.r.Types.Node(a); n.Kind == sem.KParam && n.Ent == p {
			return i, true
		}
	}
	return 0, false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
