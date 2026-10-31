package mir

import (
	"sync"

	opdef "kigumi/internal/compiler/mir/ops"
)

type executorMemo struct {
	once sync.Once
	exec bool
	hook bool
}

func (p *Program) reachMemo() *executorMemo {
	p.executor.once.Do(func() {
		rs := Reachable(p)
		for _, c := range opdef.Capabilities(rs.Ops) {
			if opdef.Capability(c) == opdef.CapTaskExecutor {
				p.executor.exec = true
			}
		}
		p.executor.hook = p.ReachesDropHook(rs)
	})
	return &p.executor
}

// ReachesTaskExecutor reports whether a build of p carries the task executor, whose runtime contains panics.
func (p *Program) ReachesTaskExecutor() bool { return p.reachMemo().exec }

// ReachesDropHook reports whether rs keeps a drop hook written in Kigumi, the only code a release can panic in.
func (p *Program) ReachesDropHook(rs *Reachability) bool {
	for id := range rs.Types {
		if d := p.R.TypeDecl(id).Drop; d != 0 && rs.Keep(d) && p.ByEnt[d] != nil {
			return true
		}
	}
	return false
}

// ContainsDropPanics reports whether a build of p runs the rest of a frame after a panic in a drop outside any task.
func (p *Program) ContainsDropPanics() bool {
	m := p.reachMemo()
	return m.exec || m.hook
}
