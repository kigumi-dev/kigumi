package vm

import (
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// Supports reports whether the VM can run every instruction and primitive
// reachable from the entry, naming the first gap otherwise, so the driver
// picks an executor before any output is produced. Reachability is by
// name for interface calls and includes every drop hook, which over-
// approximates but never misses a callee.
func Supports(prog *mir.Program) (bool, string) {
	return supportsFrom(prog, prog.Entry)
}

// ImplementationKind says how the VM runs a bodiless std function: a host
// accelerator registered under its key, a primitive the key dispatcher
// answers, or not at all (the zero value).
type ImplementationKind uint8

const (
	ImplementedHost ImplementationKind = iota + 1
	ImplementedPrimitive
)

// Implementation reports how the VM runs a bodiless std function, for the
// runtime op manifest and for Implements.
func Implementation(prog *mir.Program, fn sem.EntityID) ImplementationKind {
	m := &Machine{p: prog, r: prog.R, host: map[string]hostFn{}, sh: newShared()}
	m.registerAll()
	key := m.stdKey(fn)
	if _, hosted := m.hostForEnt(fn, key); hosted {
		return ImplementedHost
	}
	if m.primitiveKnown(key) {
		return ImplementedPrimitive
	}
	return 0
}

// Implements reports whether the VM has an implementation of a bodiless
// std function: a host accelerator or a Go primitive.
func Implements(prog *mir.Program, fn sem.EntityID) bool {
	return Implementation(prog, fn) != 0
}

// SupportsTests is Supports over every test block instead of the entry.
func SupportsTests(prog *mir.Program) (bool, string) {
	return supportsFrom(prog, testFuncs(prog)...)
}

func testFuncs(prog *mir.Program) []*midir.Func {
	var fs []*midir.Func
	for _, f := range prog.Funcs {
		if prog.R.Entity(f.Ent).Kind == sem.EntTest {
			fs = append(fs, f)
		}
	}
	return fs
}

func supportsFrom(prog *mir.Program, roots ...*midir.Func) (bool, string) {
	return newSupportIndex(prog).check(roots...)
}

// callees lists the functions an instruction can enter: the named function,
// or every method of that name when the call goes through an interface.
func (m *Machine) callees(in *midir.Inst) []*midir.Func {
	switch in.Op {
	case midir.OpCall, midir.OpClosure, midir.OpFnItem, midir.OpBind, midir.OpCFnPtr:
	default:
		return nil
	}
	if f, ok := m.p.ByEnt[in.Ent]; ok {
		// A host accelerator replaces the body, so what the body calls
		// (the C shims of std/fs, std/shell) never runs here.
		if _, hosted := m.hostFor(in.Prov, m.stdKey(in.Ent)); hosted {
			return nil
		}
		return []*midir.Func{f}
	}
	e := m.r.Entity(in.Ent)
	if e.Kind != sem.EntFn || m.r.Entity(e.Parent).Kind != sem.EntInterface || in.Op == midir.OpClosure && in.Str == "witness" {
		return nil
	}
	var fs []*midir.Func
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

func (m *Machine) supportsInst(in *midir.Inst) string {
	switch in.Op {
	case midir.OpAsm:
		return in.Op.String()
	case midir.OpCallC:
		return ""
	case midir.OpCFnPtr, midir.OpFnItem:
		e := m.r.Entity(in.Ent)
		if _, lowered := m.p.ByEnt[in.Ent]; !lowered && e.Kind == sem.EntFn && m.r.Fn(in.Ent).Abi != "" {
			return "foreign function " + e.Name
		}
		return ""
	case midir.OpCall:
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
	case midir.OpFutureNew, midir.OpAwait, midir.OpFutureDone, midir.OpCancelled, midir.OpCancelDone:
		return ""
	case midir.OpBuiltin:
		if spec, ok := opdef.BuiltinFor(in.Str); ok && spec.VM {
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
	if id, ok := opdef.StdOpFor(key); ok && id.Kind() == opdef.KindPrimitive {
		return true
	}
	if strings.HasPrefix(key, "prelude.") && (strings.Contains(strings.TrimPrefix(key, "prelude."), ".to") || strings.HasSuffix(key, ".truncate")) {
		return true
	}
	return false
}
