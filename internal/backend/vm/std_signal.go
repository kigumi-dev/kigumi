package vm

import (
	"os"
	"os/signal"
	"syscall"

	"kigumi/internal/compiler/sem"
)

// sigWatch is one signal's live watch on the VM: the channel os/signal feeds
// and how many std/os.Watch guards hold it. Native keeps the same per-signal
// depth so an inner scope ending never un-watches an outer one.
type sigWatch struct {
	ch    chan os.Signal
	depth int
}

// registerSignal gives the VM its own std/os.Signals: the Kigumi body
// reaches libc's sigaction through extern(C), which the VM statically
// refuses (OpCallC), so this drains Go's os/signal instead.
func (m *Machine) registerSignal() {
	h := m.host
	h["os.Host.signals"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Signals", nil) }
	h["os.Signals.watch"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if m.opts.DenySignals {
			m.abort("`Signals.watch` is not available under `kigumi test`")
		}
		name := m.signalName(a[1])
		m.signalChan(name)
		return m.ok(mkRecord(m.lookup("std/os", "Watch"), []*obj{mkInt(int64(signalCodeOf(name)), 32)}))
	}
	h["os.Watch.drop"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		m.unwatchSignal(int(deref(deref(a[0]).fields[0]).i))
		return unitObj
	}
	h["os.Signals.pending"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		ch, ok := m.watchedSignal(m.signalName(a[1]))
		if !ok {
			return mkBool(false)
		}
		return mkBool(drainSignal(ch))
	}
}

// signalName reads the Signal variant's tag (Interrupt, Terminate, ...).
func (m *Machine) signalName(a *obj) string { return m.r.Entity(deref(a).ent).Name }

// signalChan installs os/signal.Notify on the first watch of name and counts
// the ones already live; pending() only reports while a watch is held.
func (m *Machine) signalChan(name string) chan os.Signal {
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	if m.sh.sigWatch == nil {
		m.sh.sigWatch = map[string]*sigWatch{}
	}
	w, ok := m.sh.sigWatch[name]
	if !ok {
		w = &sigWatch{ch: make(chan os.Signal, 1)}
		// Windows has no SIGWINCH, so windowChangeSignal returns 0 and this
		// watch stays silent rather than handing signal.Notify an invalid
		// signal.
		if sig := signalOf(name); sig != syscall.Signal(0) {
			signal.Notify(w.ch, sig)
		}
		m.sh.sigWatch[name] = w
	}
	w.depth++
	return w.ch
}

// unwatchSignal drops one guard's hold; the last one stops the notification,
// so SIGINT/SIGTERM keep the Go runtime's terminating disposition again.
func (m *Machine) unwatchSignal(code int) {
	name := signalNameOfCode(code)
	if name == "" {
		return
	}
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	w, ok := m.sh.sigWatch[name]
	if !ok {
		return
	}
	if w.depth > 0 {
		w.depth--
	}
	if w.depth > 0 {
		return
	}
	signal.Stop(w.ch)
	// Go keeps its own process-wide handler installed once Notify ran; Reset
	// puts the platform default back, so SIGTERM terminates again (native's
	// rt_sys_signal_unwatch restores the sigaction it saved for the same end).
	if sig := signalOf(name); sig != syscall.Signal(0) {
		signal.Reset(sig)
	}
	for {
		select {
		case <-w.ch:
			continue
		default:
		}
		break
	}
	delete(m.sh.sigWatch, name)
}

func (m *Machine) watchedSignal(name string) (chan os.Signal, bool) {
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	w, ok := m.sh.sigWatch[name]
	if !ok {
		return nil, false
	}
	return w.ch, true
}

func signalOf(name string) os.Signal {
	switch name {
	case "Interrupt":
		return syscall.SIGINT
	case "Terminate":
		return syscall.SIGTERM
	case "WindowChange":
		return windowChangeSignal()
	default:
		return syscall.Signal(0)
	}
}

// signalCodeOf is std/os.Signal's wire discriminant, the value its
// `Signal.code` body hands the runtime.
func signalCodeOf(name string) int {
	switch name {
	case "Interrupt":
		return 0
	case "Terminate":
		return 1
	case "WindowChange":
		return 2
	default:
		return -1
	}
}

// signalNameOfCode is the inverse; rt_signal_hosted.c's kg_signal_number is
// the native twin of the pair.
func signalNameOfCode(code int) string {
	switch code {
	case 0:
		return "Interrupt"
	case 1:
		return "Terminate"
	case 2:
		return "WindowChange"
	default:
		return ""
	}
}

// drainSignal reports delivery since the last drain, clearing the whole
// queue so later calls see only new signals.
func drainSignal(ch chan os.Signal) bool {
	select {
	case <-ch:
	default:
		return false
	}
	for {
		select {
		case <-ch:
		default:
			return true
		}
	}
}
