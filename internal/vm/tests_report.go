package vm

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"kigumi/internal/diag"
	"kigumi/internal/mir"
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
}

type testCase struct {
	f    *mir.Func
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

// RunTestsReporting is RunTests under the `kigumi test` surface (--list, filter, timeouts, JSON events); it returns the number of failed and timed-out tests.
func RunTestsReporting(prog *mir.Program, opts TestRunOptions, out, errOut io.Writer) (failed int, err error) {
	cases := selectTests(prog, opts.Module, opts.Filter)
	if opts.List {
		for _, tc := range cases {
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
	var passed, skipped int
	for _, tc := range cases {
		if !deadline.IsZero() && time.Now().After(deadline) {
			skipped++
			w.emit(testEvent{Type: "testSkip", ID: tc.id.String(), File: tc.site.File, Line: tc.site.Line, Message: "suite timeout exceeded"})
			if !opts.JSON {
				fmt.Fprintf(out, "SKIP %s (suite timeout exceeded)\n", tc.id.Name)
			}
			continue
		}
		outcome, message, site, took, fresh, hardErr := runOneTest(m, tc.f, tc.id.Name, opts.PerTestTimeout)
		if hardErr != nil {
			return failed, hardErr
		}
		if fresh != nil {
			m = fresh
		}
		if outcome == "fail" || outcome == "timeout" {
			failed++
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
	w.emit(testEvent{Type: "suiteEnd", Total: len(cases), Passed: passed, Failed: failed, Skipped: skipped, DurationMS: time.Since(suiteStart).Milliseconds()})
	return failed, nil
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

// runOneTest buffers m.out/m.err privately so a timed-out test's abandoned goroutine can't race the next test's writes to the shared writers.
// A timeout returns a fresh Machine on the real writers; whatever the orphaned goroutine still writes to its buffers is lost.
func runOneTest(m *Machine, f *mir.Func, name string, timeout time.Duration) (outcome, message string, site mir.Site, took time.Duration, fresh *Machine, hardErr error) {
	realOut, realErr := m.out, m.err
	var outBuf, errBuf bytes.Buffer
	m.out, m.err = &outBuf, &errBuf

	type result struct {
		ok   bool
		cerr error
	}
	start := time.Now()
	done := make(chan result, 1)
	go func() {
		var ok bool
		cerr := m.catch(func() { ok = m.runTest(f, name) })
		done <- result{ok, cerr}
	}()

	var r result
	timedOut := false
	if timeout > 0 {
		select {
		case r = <-done:
		case <-time.After(timeout):
			timedOut = true
		}
	} else {
		r = <-done
	}
	took = time.Since(start)

	if timedOut {
		m.stopPools()
		return "timeout", fmt.Sprintf("test exceeded %s", timeout), mir.Site{}, took, newMachine(m.p, m.opts, realOut, realErr), nil
	}
	realOut.Write(outBuf.Bytes())
	realErr.Write(errBuf.Bytes())
	m.out, m.err = realOut, realErr
	if p, isPanic := r.cerr.(*Panic); isPanic {
		fmt.Fprintf(realErr, "%s: %s\n", diag.Prefix("panic"), p.Msg)
		return "fail", "panic: " + p.Msg, p.Site, took, nil, nil
	}
	if r.cerr != nil {
		return "", "", mir.Site{}, took, nil, r.cerr
	}
	if r.ok {
		return "pass", "", mir.Site{}, took, nil, nil
	}
	return "fail", errBuf.String(), mir.Site{}, took, nil, nil
}
