package vm

import "kigumi/internal/mir"

type polled int

const (
	ready polled = iota
	pending
	cancelled
)

// fn nil marks a BlockingPool.run result: its value was computed eagerly,
// and pool is the admission to release on take or drop.
type future struct {
	fn      *obj
	args    []*obj
	co      *coro
	awaited *obj
	at      int
	done    bool
	result  *obj
	pool    *obj
}

type coro struct {
	fr  frame
	bid mir.BlockID
	pc  int
}

func (m *Machine) newFuture(args []*obj) *obj {
	for _, a := range args {
		retain(a)
	}
	return mkOpaque("Future", &future{fn: args[0], args: append([]*obj{}, args[1:]...)})
}

func (m *Machine) futureOf(v *obj) *future {
	f, ok := v.data.(*future)
	if v.k != kOpaque || !ok {
		m.abort("await on a value that is not a Future")
	}
	return f
}

func (m *Machine) pollValue(v *obj) (*obj, polled) {
	d := deref(v)
	if d.k == kOpaque && d.op == "Task" {
		return m.pollTask(d.data.(*task))
	}
	return m.pollFuture(m.futureOf(d))
}

func (m *Machine) pollFuture(fu *future) (*obj, polled) {
	if fu.done {
		m.abort("a Future that already completed cannot be polled again")
	}
	if fu.fn == nil && fu.co == nil {
		return m.takeReady(fu), ready
	}
	if fu.co == nil {
		fu.co = m.startCoro(fu)
	}
	return m.resume(fu)
}

// takeReady completes a BlockingPool.run Future: its result was computed
// eagerly at run time, so the first poll only releases the pool's admission.
func (m *Machine) takeReady(fu *future) *obj {
	fu.done = true
	if fu.pool != nil {
		fu.pool.data.(*blockingPool).inFlight--
		m.release(fu.pool)
		fu.pool = nil
	}
	r := fu.result
	fu.result = nil
	return r
}

// startCoro lays out an async body's frame: the environment supplies the
// captured leading parameters and the Future's arguments the rest, moved.
func (m *Machine) startCoro(fu *future) *coro {
	fv := deref(fu.fn)
	if fv.k != kClosure || fv.fn == nil {
		m.abort("this Future has no async body to run")
	}
	var params []*obj
	nenv := 0
	if fv.env != nil {
		nenv = len(fv.env.fields)
	}
	if fv.bind {
		params = append(params, fu.args...)
	}
	for i := 0; i < nenv; i++ {
		params = append(params, retain(fv.env.fields[i]))
	}
	if !fv.bind {
		params = append(params, fu.args...)
	}
	body := fv.fn
	co := &coro{fr: frame{f: body, locals: make([]*obj, len(body.Locals))}}
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
	if m.opts.Sandbox && m.depth > m.opts.DepthLimit {
		m.abort("comptime recursion too deep")
	}
	defer func() {
		m.depth--
		m.stack = m.stack[:len(m.stack)-1]
	}()
	for {
		blk := &f.Blocks[co.bid]
		for co.pc < len(blk.Insts) {
			in := &blk.Insts[co.pc]
			if in.Op == mir.OpAwait {
				v, st := m.suspendPoint(fu, in)
				if st != ready {
					return nil, st
				}
				co.fr.locals[in.Dst] = v
				co.pc++
				continue
			}
			if m.opts.Sandbox {
				m.steps++
				if m.steps > m.opts.StepLimit {
					m.abort("comptime block took too long")
				}
			}
			m.exec(&co.fr, in)
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
			fu.done, fu.co = true, nil
			return co.fr.locals[blk.Term.Args[0]], ready
		default:
			m.abort("unreachable code in " + f.Name)
		}
	}
}
