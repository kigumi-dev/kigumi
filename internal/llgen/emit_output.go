package llgen

import "kigumi/internal/mir"

// Emit renders the whole program as one LLVM module.
func Emit(p *mir.Program) string { return EmitWith(p, EmitOptions{}) }

// Output is a compiled program: an LLVM module and, when needed, a C shim
// (emit_cshim.go).
type Output struct {
	IR    string
	CShim string
	// Capabilities are reachable Capability macros (mir.CapabilityMacros) the
	// driver passes to rt_core.c as -D flags; nil when Reachable was nil.
	Capabilities []string
	// Unwind asks for the runtime built with KIGUMI_UNWIND: the generated code registers unwind records and sweeps after a faulting drop.
	Unwind bool
	// Int128 asks for the runtime built with KIGUMI_INT128: some reachable function holds a 128-bit integer.
	Int128 bool
	// StdOps are the -D macros that compile only the rt_std_id cases and the keyed dispatch the IR calls.
	StdOps []string
	// Foreign are the C symbols the IR declares, sorted.
	Foreign []string
}

// EmitOptions selects the entry point: a hosted `main`, or `kigumi_main`
// for a freestanding target.
type EmitOptions struct {
	Freestanding bool
	// EntrySymbol names the hosted entry function; "" is `main`.
	EntrySymbol string
	// SymbolPrefix is what the target prepends to C symbol names ("_" on
	// Mach-O); module-level asm has to spell it out explicitly.
	SymbolPrefix string
	// Reachable, when set, skips a Func this reachability result does not
	// keep; nil emits every lowered function, as every direct llgen test does.
	Reachable *mir.Reachability
	// AbiCheck makes the entry verify, before anything else runs, that the linked runtime reports the fingerprint BindAbi installs.
	AbiCheck bool
	// Catch lets the build unwind on a panic; it also needs the runtime built with KIGUMI_TASK_CATCH.
	Catch bool
	// Arch and OS name the target as the driver does (amd64, riscv64, ...; linux, darwin, ...); "" is amd64 and linux.
	Arch, OS string
}

func EmitWith(p *mir.Program, opts EmitOptions) string { return EmitProgram(p, opts).IR }
