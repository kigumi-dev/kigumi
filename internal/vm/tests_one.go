package vm

import (
	"bytes"
	"fmt"
	"time"

	"kigumi/internal/diag"
	"kigumi/internal/mir"
)

// runOneTest buffers m.out/m.err so a timed-out test's abandoned goroutine can't race the next test; a timeout returns a fresh Machine.
// sink receives what the test printed per stream; jsonMode drops the stderr copy of a failure text the event message already carries.
func runOneTest(m *Machine, f *mir.Func, name string, timeout time.Duration, jsonMode bool, sink func(stream string, data []byte)) (outcome, message string, site mir.Site, took time.Duration, fresh *Machine, hardErr error) {
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
		cerr := m.catchAtUser(func() { ok = m.runTest(f, name) })
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
	m.out, m.err = realOut, realErr
	sink("stdout", outBuf.Bytes())
	if p, isPanic := r.cerr.(*Panic); isPanic {
		sink("stderr", errBuf.Bytes())
		if !jsonMode {
			sink("stderr", []byte(fmt.Sprintf("%s: %s\n", diag.Prefix("panic"), p.Msg)))
		}
		if p.InStd {
			return "fail", "panic: " + p.Msg, mir.Site{}, took, nil, nil
		}
		return "fail", "panic: " + p.Msg, p.Site, took, nil, nil
	}
	if u, isUnsupported := r.cerr.(*Unsupported); isUnsupported {
		text := u.What + " is not supported by the VM"
		if !jsonMode {
			sink("stderr", []byte(name+": "+text+"\n"))
		}
		return "fail", text, mir.Site{}, took, nil, nil
	}
	if r.cerr != nil {
		sink("stderr", errBuf.Bytes())
		return "", "", mir.Site{}, took, nil, r.cerr
	}
	if r.ok {
		sink("stderr", errBuf.Bytes())
		return "pass", "", mir.Site{}, took, nil, nil
	}
	if !jsonMode {
		sink("stderr", errBuf.Bytes())
	}
	return "fail", errBuf.String(), mir.Site{}, took, nil, nil
}
