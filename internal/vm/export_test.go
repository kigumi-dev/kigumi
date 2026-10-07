package vm

import (
	"io"

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
