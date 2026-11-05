package vm

import (
	"cmp"
	"fmt"
	"io"
	"strings"
	"time"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

// TestRunOptions configures `kigumi test`'s surface over RunTests: stable ids, --list, one filter, per-test/suite deadlines and a JSON event stream.
type TestRunOptions struct {
	Options
	// Module prefixes every test's stable id; "" omits that segment.
	Module string
	// Filter keeps only tests whose stable id contains this substring.
	Filter string
	// List prints ids (and file:line) instead of running anything.
	List bool
	// PerTestTimeout bounds one test; 0 means no bound.
	PerTestTimeout time.Duration
	// SuiteTimeout bounds the whole run; reaching it skips the rest.
	SuiteTimeout time.Duration
	// JSON streams testEvent JSON lines on out instead of ok/FAIL/TIME text.
	JSON bool
	// Unsupported renders the failure text for a test the VM cannot run; nil prints the bare gap.
	Unsupported func(why string) string
}

type testCase struct {
	f    *midir.Func
	id   TestID
	site mir.Site
}

func selectTests(prog *mir.Program, module, filter string) []testCase {
	var out []testCase
	for _, f := range testFuncs(prog) {
		tc := testCase{f: f, id: testIDOf(prog, module, f), site: prog.EntitySite(f)}
		if filter == "" || strings.Contains(tc.id.String(), filter) {
			out = append(out, tc)
		}
	}
	return out
}

// RunTestsReporting is RunTests under the `kigumi test` surface (--list, filter, timeouts, JSON events); it returns the number of failed, timed-out and suite-timeout-skipped tests, since a run that could not finish is not green.
func RunTestsReporting(prog *mir.Program, opts TestRunOptions, out, errOut io.Writer) (failed int, err error) {
	cases := selectTests(prog, opts.Module, opts.Filter)
	if len(cases) == 0 && opts.Filter != "" {
		return 0, &NoTestsMatch{Filter: opts.Filter}
	}
	if opts.List {
		w := newEventWriter(out, opts.JSON)
		for _, tc := range cases {
			if opts.JSON {
				w.emit(testEvent{Type: "testList", ID: tc.id.String(), File: tc.site.File, Line: tc.site.Line})
				continue
			}
			fmt.Fprintf(out, "%s\t%s:%d\n", tc.id, tc.site.File, tc.site.Line)
		}
		return 0, nil
	}

	w := newEventWriter(out, opts.JSON)
	w.emit(testEvent{Type: "suiteStart", Total: len(cases)})
	suiteStart := time.Now()
	var deadline time.Time
	if opts.SuiteTimeout > 0 {
		deadline = suiteStart.Add(opts.SuiteTimeout)
	}

	m := newMachine(prog, opts.Options, out, errOut)
	support := newSupportIndex(prog)
	var passed, skipped int
	var failures []testFailure
	for _, tc := range cases {
		if !deadline.IsZero() && time.Now().After(deadline) {
			skipped++
			w.emit(testEvent{Type: "testSkip", ID: tc.id.String(), File: tc.site.File, Line: tc.site.Line, Message: "suite timeout exceeded"})
			if !opts.JSON {
				fmt.Fprintf(out, "SKIP %s (suite timeout exceeded)\n", tc.id.Name)
			}
			continue
		}
		sink := func(stream string, data []byte) {
			if len(data) == 0 {
				return
			}
			switch {
			case opts.JSON:
				w.emit(testEvent{Type: "testOutput", ID: tc.id.String(), Stream: stream, Message: string(data)})
			case stream == "stderr":
				errOut.Write(data)
			default:
				out.Write(data)
			}
		}
		var outcome, message string
		var site mir.Site
		var took time.Duration
		var fresh *Machine
		var hardErr error
		if ok, why := support.check(tc.f); !ok {
			outcome, message = "fail", unsupportedText(opts, why)
			if !opts.JSON {
				sink("stderr", []byte(tc.id.Name+": "+message+"\n"))
			}
		} else {
			m.resetStatics()
			outcome, message, site, took, fresh, hardErr = runOneTest(m, tc.f, tc.id.Name, opts.PerTestTimeout, opts.JSON, sink)
		}
		if hardErr != nil {
			return failed, hardErr
		}
		if fresh != nil {
			m = fresh
		}
		if outcome == "fail" || outcome == "timeout" {
			failed++
			failures = append(failures, testFailure{id: tc.id.String(), file: cmp.Or(site.File, tc.site.File), line: cmp.Or(site.Line, tc.site.Line)})
		} else {
			passed++
		}
		// Fall back to the test declaration's site, same as --list reports.
		if site == (mir.Site{}) {
			site = tc.site
		}
		w.emit(testEvent{Type: eventType(outcome), ID: tc.id.String(), File: site.File, Line: site.Line, DurationMS: took.Milliseconds(), Message: message})
		if !opts.JSON {
			writePlainLine(out, outcome, tc.id.Name)
		}
	}
	if !opts.JSON {
		writeSummary(out, passed, failed, skipped, failures)
	}
	w.emit(testEvent{Type: "suiteEnd", Total: len(cases), Passed: passed, Failed: failed, Skipped: skipped, DurationMS: time.Since(suiteStart).Milliseconds()})
	return failed + skipped, nil
}

func unsupportedText(opts TestRunOptions, why string) string {
	if opts.Unsupported != nil {
		return opts.Unsupported(why)
	}
	return why + " is not supported by the VM"
}

func eventType(outcome string) string {
	switch outcome {
	case "pass":
		return "testPass"
	case "fail":
		return "testFail"
	case "timeout":
		return "testTimeout"
	default:
		return "test" + outcome
	}
}

func writePlainLine(out io.Writer, outcome, name string) {
	switch outcome {
	case "pass":
		fmt.Fprintf(out, "ok   %s\n", name)
	case "fail":
		fmt.Fprintf(out, "FAIL %s\n", name)
	case "timeout":
		fmt.Fprintf(out, "TIME %s\n", name)
	}
}
