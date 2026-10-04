package driver

import (
	"fmt"
	"io"
	"kigumi/internal/vm"
	"os"

	"kigumi/internal/interp"
	"kigumi/internal/sem"
)

// RunOptions is what the CLI resolved for one run.
type RunOptions struct {
	// Stdin replaces the process stdin when not nil.
	Stdin io.Reader
	// NoAccel runs std bodies in Kigumi instead of the Go accelerators.
	NoAccel bool
	// Engine selects the executor: "" or "vm" (the default) or "interp"
	// (the tree walker); any other value, "auto" included, is a CheckEngine error.
	Engine string
}

// CheckEngine rejects any Engine but "", "vm" and "interp"; the CLI's
// --engine flag validates through this too, so both ends agree.
func CheckEngine(engine string) error {
	switch engine {
	case "", "vm", "interp":
		return nil
	case "auto":
		return fmt.Errorf("\"auto\" was removed: the VM runs by default now, so there is nothing left to choose automatically; pass --engine interp for the tree walker, or run `kigumi build` for a native-only program")
	default:
		return fmt.Errorf("unknown engine %q: want vm or interp", engine)
	}
}

// Run type-checks a loaded module and interprets its entry file.
func Run(m *Module, stdout, stderr io.Writer, args []string) (int, error) {
	return RunWith(m, RunOptions{}, stdout, stderr, args)
}

// RunInput is Run with stdin replaced when stdin is not nil.
func RunInput(m *Module, stdin io.Reader, stdout, stderr io.Writer, args []string) (int, error) {
	return RunWith(m, RunOptions{Stdin: stdin}, stdout, stderr, args)
}

// RunWith picks the executor before anything runs: the VM by default (failing before output if it refuses the program) or, with --engine interp, the tree walker.
func RunWith(m *Module, opts RunOptions, stdout, stderr io.Writer, args []string) (int, error) {
	if err := CheckEngine(opts.Engine); err != nil {
		return 1, err
	}
	if opts.Engine == "interp" {
		in, err := newInterp(m, opts, stdout, stderr, args)
		if in == nil {
			return 1, err
		}
		return in.RunEntry(m.Entry), nil
	}
	a, err := Analyze(m)
	if err != nil {
		return 1, err
	}
	if a.Res.HasErrors() {
		io.WriteString(stderr, a.Res.Render())
		return 1, nil
	}
	if ok, why := vm.Supports(a.Prog); !ok {
		return 1, vmRefusalError(why)
	}
	return runVM(a, opts, stdout, stderr, args)
}

func runVM(a *Analysis, opts RunOptions, stdout, stderr io.Writer, args []string) (int, error) {
	cwd, _ := os.Getwd()
	code, err := vm.Run(a.Prog, vm.Options{Args: args, Stdin: opts.Stdin, Cwd: cwd, NoAccel: opts.NoAccel}, stdout, stderr)
	if err != nil {
		return 1, fmt.Errorf("%w (run with --engine interp)", err)
	}
	return code, nil
}

// RunTests type-checks a module loaded with tests and runs its test blocks.
func RunTests(m *Module, stdout, stderr io.Writer) (int, error) {
	return RunTestsWith(m, RunOptions{}, stdout, stderr)
}

// RunTestsWith picks the executor the way RunWith does, over the test
// blocks instead of the entry.
func RunTestsWith(m *Module, opts RunOptions, stdout, stderr io.Writer) (int, error) {
	if err := CheckEngine(opts.Engine); err != nil {
		return 1, err
	}
	if opts.Engine == "interp" {
		in, err := newInterp(m, opts, stdout, stderr, nil)
		if in == nil {
			return 1, err
		}
		in.DenySignals = true
		if in.RunTests() > 0 {
			return 1, nil
		}
		return 0, nil
	}
	a, err := Analyze(m)
	if err != nil {
		return 1, err
	}
	if a.Res.HasErrors() {
		io.WriteString(stderr, a.Res.Render())
		return 1, nil
	}
	if ok, why := vm.SupportsTests(a.Prog); !ok {
		return 1, vmRefusalError(why)
	}
	cwd, _ := os.Getwd()
	failed, err := vm.RunTests(a.Prog, vm.Options{Stdin: opts.Stdin, Cwd: cwd, NoAccel: opts.NoAccel, DenySignals: true}, stdout, stderr)
	if err != nil {
		return 1, fmt.Errorf("%w (run with --engine interp)", err)
	}
	if failed > 0 {
		return 1, nil
	}
	return 0, nil
}

// newInterp checks the module and prepares an interpreter; nil (with the
// rendered diagnostics on stderr) when the module does not check.
func newInterp(m *Module, opts RunOptions, stdout, stderr io.Writer, args []string) (*interp.Interp, error) {
	res, err := Check(m)
	if err != nil {
		return nil, err
	}
	if res.HasErrors() {
		io.WriteString(stderr, res.Render())
		return nil, nil
	}
	in := interp.New(res, stdout, stderr, args)
	in.NoAccel = opts.NoAccel
	if opts.Stdin != nil {
		in.SetStdin(opts.Stdin)
	}
	return in, nil
}

var _ = sem.Hosted
