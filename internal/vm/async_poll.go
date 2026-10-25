package vm

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

type polled int

const (
	ready polled = iota
	pending
	cancelled
)

// fn nil marks a BlockingPool.run result: its value was computed eagerly,
// and pool is the admission to release on take or drop.
type future struct {
	fn          *obj
	args        []*obj
	co          *coro
	awaited     *obj
	at          int
	done        bool
	forceCancel bool
	result      *obj
	pool        *obj
	job         *blockJob
	// yield is 1 for a fresh `task.yieldNow` Future and 2 once it has requeued its task.
	yield uint8
}

type coro struct {
	fr        frame
	bid       mir.BlockID
	pc        int
	cancelled bool
}

func (m *Machine) newFuture(args []*obj) *obj {
	for _, a := range args {
		retain(a)
	}
	return mkOpaque("Future", &future{fn: args[0], args: append([]*obj{}, args[1:]...)})
}

func (m *Machine) futureOf(v *obj) *future {
	f, ok := v.opaqueData().(*future)
	if v.k != kOpaque || !ok {
		m.abort("await on a value that is not a Future")
	}
	return f
}

// A value that is not a compiler-built Future (a BlockingPool result) has
// no mobility summary, so it stays on the lane that created it.
func (m *Machine) futureMobile(v *obj) bool {
	d := deref(v)
	if d.k != kOpaque || d.opaqueOp() != "Future" {
		return false
	}
	fu, ok := d.opaqueData().(*future)
	if !ok || fu.fn == nil {
		return false
	}
	fv := deref(fu.fn)
	if fv.k != kClosure || fv.cfn() == nil {
		return false
	}
	ent := fv.cfn().Ent
	return m.r.Entity(ent).Kind == sem.EntFn && m.r.Fn(ent).Mobility == sem.MobilityMigratable
}

func (m *Machine) pollValue(v *obj) (*obj, polled) {
	d := deref(v)
	if d.k == kOpaque && d.opaqueOp() == "Task" {
		return m.pollTask(d.opaqueData().(*task))
	}
	return m.pollFuture(m.futureOf(d))
}

func (m *Machine) pollFuture(fu *future) (*obj, polled) {
	if fu.done {
		m.abort("a Future that already completed cannot be polled again")
	}
	if fu.yield != 0 {
		return m.pollYield(fu)
	}
	if fu.job != nil {
		return m.pollJob(fu)
	}
	if fu.fn == nil && fu.co == nil {
		return m.takeReady(fu), ready
	}
	if fu.co == nil {
		fu.co = m.startCoro(fu)
	}
	return m.resume(fu)
}

// startCoro lays out an async body's frame: the environment supplies the
// captured leading parameters and the Future's arguments the rest, moved.
func (m *Machine) startCoro(fu *future) *coro {
	fv := deref(fu.fn)
	if fv.k != kClosure || fv.cfn() == nil {
		m.abort("this Future has no async body to run")
	}
	var params []*obj
	nenv := 0
	if fv.cenv() != nil {
		nenv = len(fv.cenv().fields)
	}
	if fv.bind {
		params = append(params, fu.args...)
	}
	for i := 0; i < nenv; i++ {
		params = append(params, retain(fv.cenv().fields[i]))
	}
	if !fv.bind {
		params = append(params, fu.args...)
	}
	body := fv.cfn()
	co := &coro{fr: frame{f: body, code: m.codeFor(body), locals: make([]*obj, len(body.Locals))}}
	for i, p := range body.Params {
		if i < len(params) {
			co.fr.locals[p] = params[i]
		}
	}
	fu.args = nil
	fn := fu.fn
	fu.fn = nil
	m.release(fn)
	return co
}

func (m *Machine) resume(fu *future) (*obj, polled) {
	co := fu.co
	f := co.fr.f
	m.stack = append(m.stack, f)
	m.depth++
	defer func() {
		m.depth--
		m.stack = m.stack[:len(m.stack)-1]
	}()
	m.checkDepth(f)
	m.frames = append(m.frames, &co.fr)
	leave := func() { m.frames = m.frames[:len(m.frames)-1] }
	for {
		blk := &f.Blocks[co.bid]
		if len(blk.Insts) == 0 {
			m.chargeStep()
		}
		for co.pc < len(blk.Insts) {
			in := &blk.Insts[co.pc]
			switch in.Op {
			case mir.OpAwait, mir.OpCancelled, mir.OpCancelDone:
				if st, stop := m.stepAsync(fu, in); stop {
					leave()
					return nil, st
				}
				continue
			}
			m.chargeStep()
			co.fr.blk, co.fr.pc = co.bid, co.pc
			if len(co.fr.pending) > 0 {
				m.settleSources(&co.fr, in)
			}
			m.exec(&co.fr, in, &co.fr.code.blocks[co.bid][co.pc])
			if len(co.fr.pending) > 0 {
				m.settle(&co.fr, in)
			}
			co.pc++
		}
		switch blk.Term.Op {
		case mir.TermJump:
			co.bid, co.pc = blk.Term.Targets[0], 0
		case mir.TermBranch:
			if truth(co.fr.locals[blk.Term.Args[0]]) {
				co.bid = blk.Term.Targets[0]
			} else {
				co.bid = blk.Term.Targets[1]
			}
			co.pc = 0
		case mir.TermReturn:
			if len(co.fr.pending) > 0 {
				m.settleReturn(&co.fr, blk.Term.Args[0])
			}
			fu.done, fu.co = true, nil
			leave()
			return co.fr.locals[blk.Term.Args[0]], ready
		default:
			m.abort("unreachable code in " + f.Name)
		}
	}
}
