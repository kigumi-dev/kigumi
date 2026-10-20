package vm

import (
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// Supports reports whether the VM can run every instruction and primitive
// reachable from the entry, naming the first gap otherwise, so the driver
// picks an executor before any output is produced. Reachability is by
// name for interface calls and includes every drop hook, which over-
// approximates but never misses a callee.
func Supports(prog *mir.Program) (bool, string) {
	return supportsFrom(prog, prog.Entry)
}

// Implements reports whether the VM has an implementation of a bodiless
// std function: a host accelerator or a Go primitive.
func Implements(prog *mir.Program, fn sem.EntityID) bool {
	m := &Machine{p: prog, r: prog.R, host: map[string]hostFn{}, sh: newShared()}
	m.registerAll()
	key := m.stdKey(fn)
	_, hosted := m.hostForEnt(fn, key)
	return hosted || m.primitiveKnown(key)
}

// SupportsTests is Supports over every test block instead of the entry.
func SupportsTests(prog *mir.Program) (bool, string) {
	return supportsFrom(prog, testFuncs(prog)...)
}

func testFuncs(prog *mir.Program) []*mir.Func {
	var fs []*mir.Func
	for _, f := range prog.Funcs {
		if prog.R.Entity(f.Ent).Kind == sem.EntTest {
			fs = append(fs, f)
		}
	}
	return fs
}

func supportsFrom(prog *mir.Program, roots ...*mir.Func) (bool, string) {
	m := &Machine{p: prog, r: prog.R, host: map[string]hostFn{}, sh: newShared()}
	m.registerAll()
	seen := map[*mir.Func]bool{}
	var work []*mir.Func
	add := func(f *mir.Func) {
		if f != nil && !seen[f] {
			seen[f] = true
			work = append(work, f)
		}
	}
	for _, f := range roots {
		add(f)
	}
	for ent, f := range prog.ByEnt {
		e := m.r.Entity(ent)
		if e.Kind == sem.EntFn && m.r.TypeDecl(e.Parent).Drop == ent {
			add(f)
		}
	}
	for len(work) > 0 {
		f := work[0]
		work = work[1:]
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if why := m.supportsInst(&in); why != "" {
					return false, why
				}
				for _, callee := range m.callees(&in) {
					add(callee)
				}
			}
		}
	}
	return true, ""
}

// callees lists the functions an instruction can enter: the named function,
// or every method of that name when the call goes through an interface.
func (m *Machine) callees(in *mir.Inst) []*mir.Func {
	switch in.Op {
	case mir.OpCall, mir.OpClosure, mir.OpFnItem, mir.OpBind, mir.OpCFnPtr:
	default:
		return nil
	}
	if f, ok := m.p.ByEnt[in.Ent]; ok {
		// A host accelerator replaces the body, so what the body calls
		// (the C shims of std/fs, std/shell) never runs here.
		if _, hosted := m.hostFor(in.Prov, m.stdKey(in.Ent)); hosted {
			return nil
		}
		return []*mir.Func{f}
	}
	e := m.r.Entity(in.Ent)
	if e.Kind != sem.EntFn || m.r.Entity(e.Parent).Kind != sem.EntInterface || in.Op == mir.OpClosure && in.Str == "witness" {
		return nil
	}
	var fs []*mir.Func
	for ent, f := range m.p.ByEnt {
		if m.r.Entity(ent).Name != e.Name {
			continue
		}
		// A host accelerator replaces the body here too, the same as the
		// direct-call branch above: an accelerated implementer's body
		// (e.g. its extern(C) calls) never actually runs.
		if _, hosted := m.hostForEnt(ent, m.stdKey(ent)); hosted || !m.mayWitness(in.Ent, ent) {
			continue
		}
		fs = append(fs, f)
	}
	return fs
}

func (m *Machine) supportsInst(in *mir.Inst) string {
	switch in.Op {
	case mir.OpAsm:
		return in.Op.String()
	case mir.OpCallC:
		// A CFn-typed value only ever comes from OpCFnPtr, naming a real Kigumi entity, so this is never a true foreign call.
		return ""
	case mir.OpCall:
		e := m.r.Entity(in.Ent)
		if e.Kind != sem.EntFn {
			return ""
		}
		info := m.r.Fn(in.Ent)
		if info.Abi != "" || info.Naked {
			return "foreign function " + e.Name
		}
		if _, lowered := m.p.ByEnt[in.Ent]; lowered || m.r.Entity(e.Parent).Kind == sem.EntInterface {
			return ""
		}
		key := m.stdKey(in.Ent)
		if _, ok := m.hostFor(in.Prov, key); ok || in.Prim != 0 || m.primitiveKnown(key) {
			return ""
		}
		return "primitive " + key
	case mir.OpFutureNew, mir.OpAwait, mir.OpFutureDone, mir.OpCancelled, mir.OpCancelDone:
		return ""
	case mir.OpBuiltin:
		if spec, ok := mir.BuiltinFor(in.Str); ok && spec.VM {
			return ""
		}
		if in.Str == "comptime" {
			return "an unevaluated comptime block"
		}
		return "builtin " + in.Str
	}
	return ""
}

// knownPrimitives lists branch witnesses dispatched by OpBuiltin, so they carry no registry entry of their own.
var knownPrimitives = map[string]bool{
	"prelude.Bool.branch": true, "prelude.Option.branch": true, "prelude.Result.branch": true,
}

// primitiveKnown also accepts KindPrimitive registry entries and `.to...` conversions, which callEntity lowers without a registry entry.
func (m *Machine) primitiveKnown(key string) bool {
	if knownPrimitives[key] {
		return true
	}
	if id, ok := mir.StdOpFor(key); ok && id.Kind() == mir.KindPrimitive {
		return true
	}
	if strings.HasPrefix(key, "prelude.") && strings.Contains(strings.TrimPrefix(key, "prelude."), ".to") {
		return true
	}
	return false
}
