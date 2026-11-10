package llgen

import (
	"kigumi/internal/backend/llgen/runtimegen"
	opdef "kigumi/internal/compiler/mir/ops"
)

//go:generate env UPDATE=1 go test -run TestStdOpHeaderFresh .

// StdOpHeader renders the registry as rt_std_ops.h (runtimegen.StdOpHeader).
func StdOpHeader(reg *opdef.StdOpRegistry) string { return runtimegen.StdOpHeader(runtimeOps(reg)) }

// OpMacro is the -D name that compiles the runtime case of the op named name.
func OpMacro(name string) string { return runtimegen.OpMacro(name) }

// RuntimeOps is the registry projected for the runtime generators; tests that
// join the registry with the C dispatch and the VM op table use it too.
func RuntimeOps(reg *opdef.StdOpRegistry) []runtimegen.Op { return runtimeOps(reg) }

// runtimeOps projects the registry onto the plain entries the runtime
// generators consume, so they never import compiler or MIR packages.
func runtimeOps(reg *opdef.StdOpRegistry) []runtimegen.Op {
	ops := make([]runtimegen.Op, len(reg.Ops))
	for i, op := range reg.Ops {
		var macros []string
		if op.Cap != "" {
			cap := opdef.Capability(op.Cap)
			macros = append(macros, cap.Macro())
			for _, need := range cap.Needs() {
				macros = append(macros, need.Macro())
			}
		}
		ops[i] = runtimegen.Op{ID: op.ID, Name: op.Name, Retired: op.Retired, Provider: op.Provider, Cap: string(op.Cap), CapMacros: macros, Keys: op.Keys, Suffixes: op.Suffixes, Prefixes: op.Prefixes}
	}
	return ops
}
