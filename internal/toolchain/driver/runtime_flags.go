package driver

import (
	"io"
	"slices"
	"strings"

	"kigumi/internal/backend/llgen"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/version"
)

const (
	abiMacro     = "KIGUMI_ABI_FINGERPRINT"
	threadsCap   = "KIGUMI_CAP_TASK_THREADS"
	threadsMacro = "KIGUMI_THREADS"
)

// runtimeDefines are the -D macros passed to the runtime compile for a
// build of code on t, in a fixed order.
func runtimeDefines(t buildconfig.Target, code compiled) []string {
	var defs []string
	if t.Freestanding() {
		defs = append(defs, "KIGUMI_FREESTANDING")
	}
	if t.Bare() {
		defs = append(defs, "KIGUMI_BARE")
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
func runtimeFlagSet(t buildconfig.Target, code compiled, cflags []string) []string {
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

// layoutFlagPrefixes are the C flags that move a struct's size, a field's alignment or the calling convention, so an archive compiled
// under one cannot share the runtime compiled without it. Flags that leave the layout alone (-O, -D, -I, -Wall) stay out.
var layoutFlagPrefixes = []string{
	"-fshort-", "-fpack-struct", "-freg-struct-return", "-fpcc-struct-return",
	"-funsigned-char", "-fsigned-char",
	"-malign-double", "-mno-align-double", "-mstructure-size-boundary",
	"-mabi=", "-m32", "-m64", "-mx32", "-msoft-float", "-mhard-float",
}

// layoutFlags picks the cflags that change the C data layout the runtime is compiled for.
func layoutFlags(cflags []string) []string {
	var out []string
	for _, f := range cflags {
		for _, p := range layoutFlagPrefixes {
			if strings.HasPrefix(f, p) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// runtimeFingerprint hashes what a runtime object and the code lowered against it must agree on beyond the registry and value layout:
// the macro set, the target it is compiled for and the user flags that change its layout. Two targets or two -fshort-enums builds
// therefore never share a fingerprint, so mixing their archives is E1880 instead of a silent layout disagreement.
func runtimeFingerprint(t buildconfig.Target, flags, cflags []string) string {
	inputs := append(slices.Clone(flags), "target="+targetIdentity(t))
	return llgen.CurrentAbiFingerprint(version.AbiRevision, append(inputs, layoutFlags(cflags)...))
}

// abiDefine is the -D that compiles fp into the runtime object; the quotes make the macro a string literal.
func abiDefine(fp string) string { return "-D" + abiMacro + "=\"" + fp + "\"" }

// withRuntimeABI binds the fingerprint for code built on t with cflags into its IR and records it on code.
func withRuntimeABI(t buildconfig.Target, code compiled, cflags []string) compiled {
	code.Abi = runtimeFingerprint(t, runtimeFlagSet(t, code, cflags), cflags)
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
func RuntimeProfileOf(m *Module, stderr io.Writer, t buildconfig.Target, cflags []string) (RuntimeProfile, bool, error) {
	code, ok, err := compile(m, stderr, t, emitOptionsFor(t))
	if err != nil || !ok {
		return RuntimeProfile{}, ok, err
	}
	code = withRuntimeABI(t, code, cflags)
	return RuntimeProfile{Flags: runtimeFlagSet(t, code, cflags), Fingerprint: code.Abi}, true, nil
}
