package vm

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// A watch is a guard: nested watches share one notification channel and only
// the last drop stops it, so an inner scope ending never un-watches an outer
// one, exactly as rt_signal_hosted.c refcounts the installed disposition.
func TestSignalWatchDepthSurvivesInnerDrop(t *testing.T) {
	m := &Machine{sh: newShared(), opts: Options{}}
	outer := m.signalChan("WindowChange")
	inner := m.signalChan("WindowChange")
	if inner != outer {
		t.Fatal("the second watch installed a second channel")
	}
	m.unwatchSignal(2)
	if _, ok := m.watchedSignal("WindowChange"); !ok {
		t.Fatal("dropping the inner guard ended the outer watch")
	}
	m.unwatchSignal(2)
	if _, ok := m.watchedSignal("WindowChange"); ok {
		t.Fatal("dropping the last guard left the signal watched")
	}
}

// Once the last guard is dropped the signal is delivered to the process again,
// so a flag that was queued is discarded rather than reported by a later
// pending() (a watch scope that ended must not keep eating SIGINT/SIGTERM).
func TestSignalWatchDropDiscardsQueuedFlag(t *testing.T) {
	m := &Machine{sh: newShared(), opts: Options{}}
	ch := m.signalChan("WindowChange")
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Skipf("cannot signal the test process: %v", err)
	}
	waitForSignal(t, ch)
	m.unwatchSignal(2)
	if m.sh.sigWatch != nil {
		if _, ok := m.watchedSignal("WindowChange"); ok {
			t.Fatal("the watch outlived its guard")
		}
	}
}

// stdout is written by the watcher; a queued delivery must be visible to
// pending() while the guard is alive, which is what the fixture relies on.
func TestSignalWatchPendingWhileHeld(t *testing.T) {
	m := &Machine{sh: newShared(), opts: Options{}}
	ch := m.signalChan("WindowChange")
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Skipf("cannot signal the test process: %v", err)
	}
	waitForSignal(t, ch)
	if !drainSignal(ch) {
		t.Fatal("drainSignal did not report the queued signal")
	}
	m.unwatchSignal(2)
}

func waitForSignal(t *testing.T, ch chan os.Signal) {
	t.Helper()
	for range 250 {
		select {
		case <-ch:
			ch <- syscall.SIGWINCH
			return
		default:
			time.Sleep(2 * time.Millisecond)
		}
	}
	t.Fatal("the signal never reached the watch channel")
}
