package vm

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/lang/diag"
)

// timeoutDrain bounds how long a timed-out test may keep running while the VM unwinds it: long enough to collect what
// it wrote and the panic it raised before the next cancellation point, short enough that a test parked in a blocking
// host call cannot stall the suite.
const timeoutDrain = time.Second

// drainSteps is how many instructions a timed-out test may still run while it unwinds; see shared.drainLeft.
const drainSteps = 1 << 16

// captured is one write a test made, in the order it made it: one ordered slice keeps the interleaving a captured
// stdout and stderr has, which two buffers per stream would lose.
type captured struct {
	stream string
	data   []byte
}

// recorder is the writer behind m.out and m.err while one test runs. It appends instead of buffering per stream, and
// drain detaches it so an abandoned timed-out goroutine can never write into another test's output.
type recorder struct {
	mu      sync.Mutex
	chunks  []captured
	settled bool
}

func (r *recorder) write(stream string, b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.settled {
		r.chunks = append(r.chunks, captured{stream: stream, data: append([]byte(nil), b...)})
	}
	return len(b), nil
}

// drain stops recording and hands back what the test wrote so far; later writes are dropped.
func (r *recorder) drain() []captured {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settled = true
	out := r.chunks
	r.chunks = nil
	return out
}

// text is what the test wrote to one stream, for a failure message that carries it.
func (r *recorder) text(stream string) string {
	var b strings.Builder
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.chunks {
		if c.stream == stream {
			b.Write(c.data)
		}
	}
	return b.String()
}

type streamWriter struct {
	r      *recorder
	stream string
}

func (w streamWriter) Write(b []byte) (int, error) { return w.r.write(w.stream, b) }

// testOutcome is what one test did: its verdict, the chunks it wrote, and the machine the suite continues on (a fresh
// one after a timeout, whose interpreter is abandoned).
type testOutcome struct {
	outcome string
	message string
	site    mir.Site
	took    time.Duration
	chunks  []captured
	fresh   *Machine
	hardErr error
}

// runOneTest runs one test block with m and reports what it did. A timeout is a verdict, not a stop: the interpreter may
// finish the statement it is in, so what it wrote and the panic it raised are still reported, but its next host effect is
// refused and a bounded instruction budget ends anything that would outlive the verdict.
func runOneTest(m *Machine, f *midir.Func, name string, timeout time.Duration, jsonMode bool) testOutcome {
	realOut, realErr := m.out, m.err
	rec := &recorder{}
	m.out, m.err = streamWriter{rec, "stdout"}, streamWriter{rec, "stderr"}

	type result struct {
		ok   bool
		cerr error
	}
	start := time.Now()
	done := make(chan result, 1)
	go func() {
		var ok bool
		var cerr error
		func() {
			// The timeout path cancels the interpreter, whose cancellation point panics: recovery belongs to the
			// abandoned goroutine, not to the process.
			defer func() {
				if rec := recover(); rec != nil {
					if cancelled, isCancel := rec.(*Cancelled); isCancel {
						cerr = cancelled
						return
					}
					panic(rec)
				}
			}()
			cerr = m.catchAtUser(func() { ok = m.runTest(f, name) })
		}()
		done <- result{ok, cerr}
	}()

	var r result
	var settled, timedOut bool
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		select {
		case r = <-done:
			settled = true
			timer.Stop()
		case <-timer.C:
			timedOut = true
			// Deterministic and immediate: the test may finish the statement it is in and report it, but the next
			// host effect is refused. The instruction budget bounds the rest.
			m.sh.effectsOff.Store(true)
			m.sh.drainLeft.Store(drainSteps)
			m.stopPools()
			select {
			case r = <-done:
				settled = true
			case <-time.After(timeoutDrain):
			}
		}
	} else {
		r = <-done
		settled = true
	}
	took := time.Since(start)
	if !timedOut {
		m.out, m.err = realOut, realErr
	}

	if timedOut {
		out := testOutcome{
			outcome: "timeout",
			message: fmt.Sprintf("test exceeded %s", timeout),
			took:    took,
			fresh:   newMachine(m.p, m.opts, realOut, realErr),
		}
		if settled {
			if p, isPanic := r.cerr.(*Panic); isPanic {
				out.message += ": panic: " + p.Msg
				out.site = p.Site
				if !jsonMode {
					rec.write("stderr", []byte(fmt.Sprintf("%s: %s\n", diag.Prefix("panic"), p.Msg)))
				}
			} else if r.cerr == nil && r.ok {
				out.message += "; the test finished while unwinding"
			}
		}
		out.chunks = rec.drain()
		return out
	}

	out := testOutcome{took: took}
	switch {
	case isPanic(r.cerr):
		p := r.cerr.(*Panic)
		if !jsonMode {
			rec.write("stderr", []byte(fmt.Sprintf("%s: %s\n", diag.Prefix("panic"), p.Msg)))
		}
		out.outcome, out.message, out.site = "fail", "panic: "+p.Msg, p.Site
	case isUnsupported(r.cerr):
		u := r.cerr.(*Unsupported)
		text := u.What + " is not supported by the VM"
		if !jsonMode {
			rec.write("stderr", []byte(name+": "+text+"\n"))
		}
		out.outcome, out.message = "fail", text
	case r.cerr != nil:
		out.hardErr = r.cerr
	case r.ok:
		out.outcome = "pass"
	default:
		out.outcome, out.message = "fail", rec.text("stderr")
	}
	out.chunks = rec.drain()
	// A failing test's stderr is its failure message; JSON carries it once, in the event's message.
	if out.outcome == "fail" && jsonMode && !isPanic(r.cerr) && !isUnsupported(r.cerr) {
		out.chunks = withoutStream(out.chunks, "stderr")
	}
	return out
}

func isPanic(err error) bool       { _, ok := err.(*Panic); return ok }
func isUnsupported(err error) bool { _, ok := err.(*Unsupported); return ok }

func withoutStream(chunks []captured, stream string) []captured {
	kept := chunks[:0]
	for _, c := range chunks {
		if c.stream != stream {
			kept = append(kept, c)
		}
	}
	return kept
}
