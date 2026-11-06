package vm

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// frame is one activation: its locals.
type frame struct {
	f       *midir.Func
	code    *funcCode
	locals  []*obj
	pending []crossing
	blk     midir.BlockID
	pc      int
	// A frame with no f stands for a call's references: a sweep releases them where the call sat on the stack.
	held []*obj
}

// call runs a MIR function on arguments handed over (owned) or lent.
func (m *Machine) call(f *midir.Func, args []*obj) *obj {
	fr := m.newFrame(f, m.codeFor(f))
	for i, p := range f.Params {
		if i < len(args) {
			fr.locals[p] = args[i]
		}
	}
	return m.run(fr)
}

// callFrom is call with the arguments read straight from the caller's locals.
func (m *Machine) callFrom(f *midir.Func, code *funcCode, caller []*obj, args []midir.LocalID) *obj {
	fr := m.newFrame(f, code)
	for i, p := range f.Params {
		if i < len(args) {
			fr.locals[p] = caller[args[i]]
		}
	}
	return m.run(fr)
}

func (m *Machine) newFrame(f *midir.Func, code *funcCode) *frame {
	var fr *frame
	if n := len(m.spare); n > 0 {
		fr = m.spare[n-1]
		m.spare = m.spare[:n-1]
	} else {
		fr = new(frame)
	}
	if n := len(f.Locals); cap(fr.locals) >= n {
		fr.locals = fr.locals[:n]
	} else {
		fr.locals = make([]*obj, n)
	}
	fr.f, fr.code = f, code
	return fr
}

// recycle keeps a returned frame for the next call; every slot of its capacity is nil again.
func (m *Machine) recycle(fr *frame) {
	clear(fr.locals)
	fr.pending = fr.pending[:0]
	fr.f, fr.code, fr.blk, fr.pc = nil, nil, 0, 0
	m.spare = append(m.spare, fr)
}

func (m *Machine) run(fr *frame) *obj {
	f := fr.f
	m.stack = append(m.stack, f)
	m.frames = append(m.frames, fr)
	m.depth++
	defer func() {
		m.depth--
		m.stack = m.stack[:len(m.stack)-1]
	}()
	m.checkDepth(f)
	bid := midir.BlockID(0)
	for {
		blk := &f.Blocks[bid]
		ics := fr.code.blocks[bid]
		m.traceBlock(f, bid)
		// An empty loop still has to run into the step limit.
		if len(blk.Insts) == 0 {
			m.chargeStep()
		}
		for k := range blk.Insts {
			m.chargeStep()
			fr.blk, fr.pc = bid, k
			if len(fr.pending) > 0 {
				m.settleSources(fr, &blk.Insts[k])
			}
			m.exec(fr, &blk.Insts[k], &ics[k])
			if len(fr.pending) > 0 {
				m.settle(fr, &blk.Insts[k])
			}
		}
		switch blk.Term.Op {
		case midir.TermJump:
			bid = blk.Term.Targets[0]
		case midir.TermBranch:
			if m.opts.SecretSan {
				m.checkBranch(fr.locals[blk.Term.Args[0]])
			}
			c := truth(fr.locals[blk.Term.Args[0]])
			if c {
				bid = blk.Term.Targets[0]
			} else {
				bid = blk.Term.Targets[1]
			}
		case midir.TermReturn:
			if len(fr.pending) > 0 {
				m.settleReturn(fr, blk.Term.Args[0])
			}
			m.frames = m.frames[:len(m.frames)-1]
			res := fr.locals[blk.Term.Args[0]]
			m.recycle(fr)
			return res
		default:
			m.abort("unreachable code in " + f.Name)
		}
	}
}

func truth(v *obj) bool {
	v = deref(v)
	return v != nil && v.k == kBool && v.b
}
