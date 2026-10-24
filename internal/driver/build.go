package driver

import (
	"io"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
)

// Compile type-checks and lowers m to MIR, returning the LLVM IR a default BuildWith for m's own target links.
func Compile(m *Module, stderr io.Writer) (string, bool, error) {
	return CompileWith(m, stderr, BuildOptions{Target: m.target})
}

// CompileWith is the IR BuildWith links under opts, Header records included, so --emit-llvm matches the binary.
func CompileWith(m *Module, stderr io.Writer, opts BuildOptions) (string, bool, error) {
	l, ok, err := compileLinked(m, stderr, opts, true)
	return l.code.IR, ok, err
}

// emitOptionsFor is the one Target -> llgen.EmitOptions mapping shared by
// IR dump (Compile), a normal build and the build graph (buildOnce).
func emitOptionsFor(t Target) llgen.EmitOptions {
	opts := llgen.EmitOptions{Freestanding: t.Bare(), Catch: wantsTaskCatch(t), AbiCheck: true}
	if t.OS == "darwin" {
		opts.SymbolPrefix = "_"
	}
	if t.OS == "wasi" {
		opts.EntrySymbol = "__main_argc_argv"
	}
	return opts
}

type compiled struct {
	llgen.Output
	Reasons []mir.CapabilityReason
	// Abi is the runtime fingerprint bound into the IR; empty until withRuntimeABI runs.
	Abi string
}

func compile(m *Module, stderr io.Writer, t Target, opts llgen.EmitOptions) (compiled, bool, error) {
	if m.ReportDiagnostics(stderr) {
		return compiled{}, false, nil
	}
	a, err := Analyze(m)
	if err != nil {
		return compiled{}, false, err
	}
	if a.Res.HasErrors() {
		io.WriteString(stderr, a.Res.Render())
		return compiled{}, false, nil
	}
	reach := a.checkProvision(t)
	if a.Res.HasErrors() {
		io.WriteString(stderr, a.Res.Render())
		return compiled{}, false, nil
	}
	io.WriteString(stderr, a.Res.RenderTaskPanicWarning())
	opts.Reachable = reach
	m.stats.Emit++
	return compiled{Output: llgen.EmitProgram(a.Prog, opts), Reasons: reach.CapabilityReasons(a.Res, t.CapabilityPlatform())}, true, nil
}

// BuildOptions selects the target; the zero value builds for the host.
type BuildOptions struct {
	Target Target
	// CFlags are extra C compiler/linker args (CLI passes KIGUMI_CFLAGS); only its compiler-shaped tokens reach the archive path.
	CFlags []string
	// LinkerScript, LinkFlags and Entry come from a build program's Artifact fields and apply to a hosted link only, never the archive path.
	LinkerScript string
	LinkFlags    []string
	Entry        string
	// Reproducible builds m twice under buildReproducible instead of once,
	// for the --reproducible flag.
	Reproducible bool
	// Env (from shared.CCEnv) is the C compiler environment Reproducible's two internal builds run in; internal/driver never reads the ambient env.
	Env []string
	// Link is the --link choice for an executable built for the host.
	Link LinkMode
	// SizeMin is --size-min: -Os, LTO, no unwind tables and a stripped executable.
	SizeMin bool
	// Report, when set, receives what the build resolved for the build summary.
	Report *BuildReport
	// pinCompDir and ccEnv are set only by buildReproducible's nested builds, for the cmd.Dir treatment buildArchive always uses; a plain build leaves both zero.
	pinCompDir bool
	ccEnv      []string
}

// Build links a host executable at exe.
func Build(m *Module, exe string, stderr io.Writer) (bool, error) {
	return BuildWith(m, exe, stderr, BuildOptions{})
}

// BuildWith writes the IR and runtime into a work directory and produces out: an executable, or for freestanding a kigumi_main static archive.
func BuildWith(m *Module, out string, stderr io.Writer, opts BuildOptions) (bool, error) {
	out, err := m.compiler.Abs(out)
	if err != nil {
		return false, err
	}
	if opts.Reproducible {
		return buildReproducible(m, out, stderr, opts)
	}
	return buildOnce(m, out, stderr, opts)
}
