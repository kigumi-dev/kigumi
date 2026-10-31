package driver

import (
	"io"
	"slices"
	"strings"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/toolchain/version"
)

const (
	abiMacro     = "KIGUMI_ABI_FINGERPRINT"
	threadsCap   = "KIGUMI_CAP_TASK_THREADS"
	threadsMacro = "KIGUMI_THREADS"
)

// runtimeDefines are the -D macros passed to the runtime compile for a
// build of code on t, in a fixed order.
func runtimeDefines(t Target, code compiled) []string {
	var defs []string
	if t.Freestanding() {
		defs = append(defs, "KIGUMI_FREESTANDING")
	}
	if wantsTaskCatch(t) {
		defs = append(defs, "KIGUMI_TASK_CATCH")
	}
	if code.Unwind {
		defs = append(defs, "KIGUMI_UNWIND")
	}
	if code.Int128 {
		defs = append(defs, "KIGUMI_INT128")
	}
	if bareJumpTarget(t) {
		defs = append(defs, "KIGUMI_TASK_CATCH_BUILTIN")
	}
	defs = append(defs, code.Capabilities...)
	if t.COFF() {
		defs = append(defs, "KIGUMI_COFF")
	}
	return defs
}

// runtimeFlagSet is every KIGUMI_ macro the runtime is compiled under: the driver's
// defines, KIGUMI_THREADS (rt.h derives it from task.threads), the user's -D flags and any -fsanitize (TSan changes the atomic path).
func runtimeFlagSet(t Target, code compiled, cflags []string) []string {
	flags := runtimeDefines(t, code)
	if slices.Contains(flags, threadsCap) {
		flags = append(flags, threadsMacro)
	}
	for _, f := range cflags {
		name, _, _ := strings.Cut(strings.TrimPrefix(f, "-D"), "=")
		switch {
		case strings.HasPrefix(f, "-D") && strings.HasPrefix(name, "KIGUMI_") && name != abiMacro:
			flags = append(flags, strings.TrimPrefix(f, "-D"))
		case strings.HasPrefix(f, "-fsanitize"):
			flags = append(flags, f)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(flags)))
}

// abiDefine is the -D that compiles fp into the runtime object; the quotes make the macro a string literal.
func abiDefine(fp string) string { return "-D" + abiMacro + "=\"" + fp + "\"" }

// withRuntimeABI binds the fingerprint for code built on t with cflags into its IR and records it on code.
func withRuntimeABI(t Target, code compiled, cflags []string) compiled {
	code.Abi = llgen.CurrentAbiFingerprint(version.AbiRevision, runtimeFlagSet(t, code, cflags))
	code.Output = code.Output.BindAbi(code.Abi)
	return code
}

// RuntimeProfile is the runtime object a build of a module would compile:
// the KIGUMI_ macros it is built under and the fingerprint they and the registry and layout produce.
type RuntimeProfile struct {
	Flags       []string
	Fingerprint string
}

// RuntimeProfileOf analyzes and lowers m for t without invoking a C compiler.
func RuntimeProfileOf(m *Module, stderr io.Writer, t Target, cflags []string) (RuntimeProfile, bool, error) {
	code, ok, err := compile(m, stderr, t, emitOptionsFor(t))
	if err != nil || !ok {
		return RuntimeProfile{}, ok, err
	}
	code = withRuntimeABI(t, code, cflags)
	return RuntimeProfile{Flags: runtimeFlagSet(t, code, cflags), Fingerprint: code.Abi}, true, nil
}
