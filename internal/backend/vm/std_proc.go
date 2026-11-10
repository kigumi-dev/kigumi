package vm

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"kigumi/internal/compiler/sem"
)

// vmProcess is one child the VM spawned. A goroutine waits on the child as
// soon as it starts, so the kernel reaps it even when the program drops the
// handle without waiting; done publishes the result to the waiting host ops.
type vmProcess struct {
	cmd  *os.Process
	done chan struct{}
	// state and waitErr are written before done is closed and only read after
	// receiving from it, so the closed channel supplies the happens-before edge.
	state   *os.ProcessState
	waitErr error
}

// procOf returns the live child with pid. The caller must not hold procMu.
func (m *Machine) procOf(pid int64) (*vmProcess, bool) {
	m.sh.procMu.Lock()
	defer m.sh.procMu.Unlock()
	vp, ok := m.sh.procs[pid]
	return vp, ok
}

// forgetProc removes pid's table entry once its status is recorded on the
// handle; a repeated wait then uses the handle's cached status.
func (m *Machine) forgetProc(pid int64) {
	m.sh.procMu.Lock()
	delete(m.sh.procs, pid)
	m.sh.procMu.Unlock()
}

// procExitStatus is the wait status std/proc reports: the exit code, or
// 128 + signal number when a signal ended the child.
func procExitStatus(st *os.ProcessState) int64 {
	code := int64(st.ExitCode())
	if code < 0 {
		if ws, ok := st.Sys().(syscall.WaitStatus); ok {
			code = 128 + int64(ws.Signal())
		}
	}
	return code
}

// recordProcStatus stores a reaped status on the handle record.
func (m *Machine) recordProcStatus(rec *obj, code int64) {
	rec.fields[1] = mkBool(true)
	rec.fields[2] = mkInt(code, m.numKind(sem.TyI64))
}

// registerProc supplies std/proc's host operations. The Kigumi body calls
// fork/execvp for native builds; the VM runs the same operations through Go.
// tryWait is the piece the shared std/proc body needs so timeout and
// cancellation can poll instead of blocking the lane.
func (m *Machine) registerProc() {
	if m.host == nil {
		return
	}
	m.host["proc.Processes.spawn"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		program := string(deref(a[1]).str())
		args := deref(a[2]).fields
		argv := make([]string, 0, len(args))
		for _, arg := range args {
			argv = append(argv, string(deref(arg).str()))
		}
		cmd := exec.Command(program, argv...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return m.errMsg(err.Error())
		}
		vp := &vmProcess{cmd: cmd.Process, done: make(chan struct{})}
		go func() {
			vp.state, vp.waitErr = vp.cmd.Wait()
			close(vp.done)
		}()
		pid := int64(cmd.Process.Pid)
		m.sh.procMu.Lock()
		if m.sh.procs == nil {
			m.sh.procs = map[int64]*vmProcess{}
		}
		m.sh.procs[pid] = vp
		m.sh.procMu.Unlock()
		rec := mkRecord(m.lookup("std/proc", "Process"), []*obj{
			mkInt(pid, m.numKind(sem.TyI32)),
			mkBool(false),
			mkInt(0, m.numKind(sem.TyI64)),
		})
		return m.ok(rec)
	}
	m.host["proc.Process.wait"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		rec := deref(a[0])
		if rec.fields[1].b {
			return m.ok(mkInt(rec.fields[2].i, m.numKind(sem.TyI64)))
		}
		pid := deref(rec.fields[0]).i
		vp, ok := m.procOf(pid)
		if !ok {
			return m.errMsg("process wait: unknown process")
		}
		<-vp.done
		if vp.waitErr != nil {
			return m.errMsg("process wait failed: " + vp.waitErr.Error())
		}
		m.forgetProc(pid)
		code := procExitStatus(vp.state)
		m.recordProcStatus(rec, code)
		return m.ok(mkInt(code, m.numKind(sem.TyI64)))
	}
	m.host["proc.Process.tryWait"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		rec := deref(a[0])
		if rec.fields[1].b {
			return m.ok(m.some(mkInt(rec.fields[2].i, m.numKind(sem.TyI64))))
		}
		pid := deref(rec.fields[0]).i
		vp, ok := m.procOf(pid)
		if !ok {
			return m.errMsg("process wait: unknown process")
		}
		select {
		case <-vp.done:
			if vp.waitErr != nil {
				return m.errMsg("process wait failed: " + vp.waitErr.Error())
			}
			m.forgetProc(pid)
			code := procExitStatus(vp.state)
			m.recordProcStatus(rec, code)
			return m.ok(m.some(mkInt(code, m.numKind(sem.TyI64))))
		default:
			return m.ok(m.none())
		}
	}
	m.host["proc.Process.signal"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		rec := deref(a[0])
		if rec.fields[1].b {
			return m.ok(unitObj)
		}
		pid := deref(rec.fields[0]).i
		vp, ok := m.procOf(pid)
		if !ok {
			return m.errMsg("process signal: unknown process")
		}
		name := m.r.Entity(deref(a[1]).ent).Name
		var sig syscall.Signal
		switch name {
		case "Interrupt":
			sig = syscall.SIGINT
		case "Terminate":
			sig = syscall.SIGTERM
		case "Kill":
			sig = syscall.SIGKILL
		default:
			return m.errMsg("process signal: unknown signal " + name)
		}
		if err := vp.cmd.Signal(sig); err != nil {
			// A child that exited between the handle check and the signal is
			// already gone; ESRCH (or Go's equivalent) is not a failure.
			if !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
				return m.errMsg("process signal failed: " + err.Error())
			}
		}
		return m.ok(unitObj)
	}
}
