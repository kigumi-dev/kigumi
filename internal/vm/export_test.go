package vm

import (
	"fmt"
	"io"

	"kigumi/internal/diag"
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func RunSandboxedPool(prog *mir.Program, opts Options, out, errOut io.Writer) (code int, steps int64, err error) {
	m := newMachine(prog, opts, out, errOut)
	m.host["task.threadPoolExecutor"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		e := m.newExecutor(int(deref(a[0]).i))
		m.startWorkers(e)
		return mkOpaque("Executor", e)
	}
	m.indexHostByOp()
	code, err = runMachine(m, prog, errOut)
	return code, m.sh.steps.Load(), err
}

const StepBatch = stepBatch

func TrackLive(on bool) {
	liveTrack.Store(on)
	liveMade.Store(0)
	liveFreed.Store(0)
}

func PauseLive(on bool) { liveTrack.Store(on) }

func LiveObjects() int64 { return liveMade.Load() - liveFreed.Load() }

// RunReleasing is Run that also gives back the entry's result, which a real
// run leaves to the process exit; atExit runs just before that.
func RunReleasing(prog *mir.Program, opts Options, out, errOut io.Writer, atExit func()) (int, error) {
	m := newMachine(prog, opts, out, errOut)
	var code int
	err := m.catch(func() {
		res := m.call(prog.Entry, nil)
		code = m.exitCode(res)
		atExit()
		m.release(res)
	})
	if p, ok := err.(*Panic); ok {
		fmt.Fprintf(errOut, "%s: %s\n", diag.Prefix("panic"), p.Msg)
		return 2, nil
	}
	return code, err
}
