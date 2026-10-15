package driver

import (
	"fmt"
	"io"
	"kigumi/internal/vm"
	"os"
)

// RunOptions is what the CLI resolved for one run.
type RunOptions struct {
	// Stdin replaces the process stdin when not nil.
	Stdin io.Reader
	// NoAccel runs std bodies in Kigumi instead of the Go accelerators.
	NoAccel bool
	// FaultInjectAt is a test-only hook; see vm.Options.FaultInjectAt.
	FaultInjectAt int64
	// Cwd is the directory relative paths resolve against; empty means the process directory.
	Cwd string
}

func (o RunOptions) cwd() string {
	if o.Cwd != "" {
		return o.Cwd
	}
	cwd, _ := os.Getwd()
	return cwd
}

// Run type-checks a loaded module and runs its entry file on the VM.
func Run(m *Module, stdout, stderr io.Writer, args []string) (int, error) {
	return RunWith(m, RunOptions{}, stdout, stderr, args)
}

// RunInput is Run with stdin replaced when stdin is not nil.
func RunInput(m *Module, stdin io.Reader, stdout, stderr io.Writer, args []string) (int, error) {
	return RunWith(m, RunOptions{Stdin: stdin}, stdout, stderr, args)
}

// RunWith type-checks m and runs its entry on the VM, failing before any
// output if the VM refuses the program.
func RunWith(m *Module, opts RunOptions, stdout, stderr io.Writer, args []string) (int, error) {
	a, err := Analyze(m)
	if err != nil {
		return 1, err
	}
	if a.Res.HasErrors() {
		io.WriteString(stderr, a.Res.Render())
		return 1, nil
	}
	if err := VMRefusal(a); err != nil {
		return 1, err
	}
	return runVM(a, opts, stdout, stderr, args)
}

func runVM(a *Analysis, opts RunOptions, stdout, stderr io.Writer, args []string) (int, error) {
	code, err := vm.Run(a.Prog, vm.Options{Args: args, Stdin: opts.Stdin, Cwd: opts.cwd(), NoAccel: opts.NoAccel, FaultInjectAt: opts.FaultInjectAt}, stdout, stderr)
	if err != nil {
		return 1, fmt.Errorf("%w", err)
	}
	return code, nil
}

// RunTests type-checks a module loaded with tests and runs its test blocks on the VM.
func RunTests(m *Module, stdout, stderr io.Writer) (int, error) {
	return RunTestsWith(m, RunOptions{}, stdout, stderr)
}

// RunTestsWith is RunTests with explicit options.
func RunTestsWith(m *Module, opts RunOptions, stdout, stderr io.Writer) (int, error) {
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
	failed, err := vm.RunTests(a.Prog, vm.Options{Stdin: opts.Stdin, Cwd: opts.cwd(), NoAccel: opts.NoAccel, DenySignals: true}, stdout, stderr)
	if err != nil {
		return 1, fmt.Errorf("%w", err)
	}
	if failed > 0 {
		return 1, nil
	}
	return 0, nil
}
