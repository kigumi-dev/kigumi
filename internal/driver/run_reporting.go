package driver

import (
	"fmt"
	"io"
	"time"

	"kigumi/internal/vm"
)

// TestEventFormatVersion re-exports vm.TestEventFormatVersion: version
// cannot import vm (layering), so cli/doctor reads it through driver instead.
const TestEventFormatVersion = vm.TestEventFormatVersion

// TestRunOptions is RunOptions plus `kigumi test`'s reporting surface:
// stable ids, --list, one filter, per-test/suite timeouts and a JSON event stream.
type TestRunOptions struct {
	RunOptions
	// Filter keeps only tests whose stable id (<module>/<package>::<name>)
	// contains this substring.
	Filter string
	// List prints every selected id with its file:line and runs nothing.
	List bool
	// PerTestTimeout bounds one test; 0 means no bound.
	PerTestTimeout time.Duration
	// SuiteTimeout bounds the whole run; reaching it skips the rest.
	SuiteTimeout time.Duration
	// JSON streams versioned testEvent JSON lines instead of ok/FAIL/TIME text.
	JSON bool
}

// RunTestsReportingWith is RunTestsWith with the list/filter/timeout/JSON surface.
func RunTestsReportingWith(m *Module, opts TestRunOptions, stdout, stderr io.Writer) (int, error) {
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
	failed, err := vm.RunTestsReporting(a.Prog, vm.TestRunOptions{
		Options:        vm.Options{Stdin: opts.Stdin, Cwd: m.cwdFor(opts.Cwd), NoAccel: opts.NoAccel, DenySignals: true},
		Module:         m.Name,
		Filter:         opts.Filter,
		List:           opts.List,
		PerTestTimeout: opts.PerTestTimeout,
		SuiteTimeout:   opts.SuiteTimeout,
		JSON:           opts.JSON,
	}, stdout, stderr)
	if err != nil {
		return 1, fmt.Errorf("%w (run with --engine interp)", err)
	}
	if failed > 0 {
		return 1, nil
	}
	return 0, nil
}
