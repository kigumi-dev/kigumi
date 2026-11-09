package vm

import (
	"os"
	"os/exec"
	"syscall"

	"kigumi/internal/compiler/sem"
)

// registerProc supplies std/proc's host operations. The Kigumi body calls
// fork/execvp for native builds; the VM runs the same operations through Go.
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
		cmd.Stdout = m.out
		cmd.Stderr = m.err
		if err := cmd.Start(); err != nil {
			return m.errMsg(err.Error())
		}
		pid := int64(cmd.Process.Pid)
		if m.procs == nil {
			m.procs = map[int64]*os.Process{}
		}
		m.procs[pid] = cmd.Process
		rec := mkRecord(m.lookup("std/proc", "Process"), []*obj{
			mkInt(pid, m.numKind(sem.TyI32)),
			mkBool(false),
			mkInt(0, m.numKind(sem.TyI64)),
		})
		return m.ok(rec)
	}
	m.host["proc.Process.wait"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		rec := deref(a[0])
		pid := deref(rec.fields[0]).i
		if rec.fields[1].b {
			return m.ok(mkInt(rec.fields[2].i, m.numKind(sem.TyI64)))
		}
		proc := m.procs[pid]
		if proc == nil {
			return m.errMsg("process wait: unknown process")
		}
		state, err := proc.Wait()
		delete(m.procs, pid)
		if err != nil {
			return m.errMsg("process wait failed: " + err.Error())
		}
		code := int64(state.ExitCode())
		if code < 0 {
			if ws, ok := state.Sys().(interface{ Signal() syscall.Signal }); ok {
				code = 128 + int64(ws.Signal())
			}
		}
		rec.fields[1] = mkBool(true)
		rec.fields[2] = mkInt(code, m.numKind(sem.TyI64))
		return m.ok(mkInt(code, m.numKind(sem.TyI64)))
	}
	m.host["proc.Process.signal"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		rec := deref(a[0])
		if rec.fields[1].b {
			return m.ok(unitObj)
		}
		pid := deref(rec.fields[0]).i
		proc := m.procs[pid]
		if proc == nil {
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
		if err := proc.Signal(sig); err != nil {
			return m.errMsg("process signal failed: " + err.Error())
		}
		return m.ok(unitObj)
	}
}
