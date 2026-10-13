package driver_test

import (
	"fmt"
	"slices"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

var sameArity = [][]mir.Opcode{
	{mir.OpCopy, mir.OpMove, mir.OpField, mir.OpFieldMove, mir.OpTag, mir.OpPayload, mir.OpIsVariant, mir.OpIsType, mir.OpUnbox, mir.OpBox, mir.OpBorrow, mir.OpNewCell, mir.OpCellGet, mir.OpCellTake, mir.OpAlias, mir.OpShare, mir.OpBoxDirect, mir.OpUnboxDirect, mir.OpAwait, mir.OpUnary},
	{mir.OpBinary, mir.OpIndex},
	{mir.OpSetField, mir.OpCellSet},
	{mir.OpDrop, mir.OpRelease, mir.OpFutureDone, mir.OpGlobalSet},
	{mir.OpConst, mir.OpUnit, mir.OpFnItem, mir.OpCFnPtr, mir.OpGlobalGet},
	{mir.OpPanic, mir.OpNop},
	{mir.OpCall, mir.OpCallValue, mir.OpBuiltin, mir.OpRecord, mir.OpVariant, mir.OpArray, mir.OpClosure, mir.OpBind, mir.OpCallC, mir.OpAsm, mir.OpInterp, mir.OpShell, mir.OpFutureNew},
	{mir.OpSetIndex},
}

var allEdits = []edit{
	{"operand", true, editOperand},
	{"dst", true, editDst},
	{"opcode-any", true, editOpcodeAny},
	{"opcode-arity", true, editOpcodeArity},
	{"remove", true, editRemove},
	{"duplicate", true, editDuplicate},
	{"swap-insts", true, editSwapInsts},
	{"insert", true, editInsert},
	{"arg-drop", true, editArgDrop},
	{"arg-dup", true, editArgDup},
	{"arg-add", true, editArgAdd},
	{"arg-swap", true, editArgSwap},
	{"type", true, editInstType},
	{"const-type", true, editConstType},
	{"entity", true, editEntity},
	{"redirect-call", true, editRedirectCall},
	{"index", true, editIndex},
	{"text", true, editStr},
	{"flag", true, editFlag},
	{"prim", true, editPrim},
	{"literal", true, editLiteral},
	{"local-type", false, editLocalType},
	{"local-flag", false, editLocalFlag},
	{"direct-flag", false, editDirectFlag},
	{"direct-sig", false, editDirectSig},
	{"term-remove", false, editTermRemove},
	{"term-target", false, editTermTarget},
	{"term-swap-edge", false, editTermSwapEdge},
	{"term-kind", false, editTermKind},
	{"term-operand", false, editTermOperand},
	{"func", false, editFunc},
}

func editOperand(s *site) string {
	in := s.inst()
	if len(in.Args) == 0 {
		return ""
	}
	k := s.r.Intn(len(in.Args))
	in.Args[k] = s.local()
	return fmt.Sprintf("%s operand %d of %s", s.where(), k, in.Op)
}

func editDst(s *site) string {
	in := s.inst()
	in.Dst = s.local()
	return fmt.Sprintf("%s destination of %s", s.where(), in.Op)
}

func editOpcodeAny(s *site) string {
	in := s.inst()
	old := in.Op
	in.Op = mir.Opcode(s.r.Intn(int(mir.OpUnboxDirect)) + 1)
	return fmt.Sprintf("%s %s to %s", s.where(), old, in.Op)
}

func editOpcodeArity(s *site) string {
	in := s.inst()
	for _, fam := range sameArity {
		if !slices.Contains(fam, in.Op) {
			continue
		}
		old := in.Op
		in.Op = fam[s.r.Intn(len(fam))]
		if in.Op == old {
			return ""
		}
		return fmt.Sprintf("%s %s to %s", s.where(), old, in.Op)
	}
	return ""
}

func editRemove(s *site) string {
	b := s.blk()
	b.Insts = slices.Delete(b.Insts, s.ii, s.ii+1)
	return s.where() + " removed"
}

func editDuplicate(s *site) string {
	b := s.blk()
	in := b.Insts[s.ii]
	in.Args = slices.Clone(in.Args)
	b.Insts = slices.Insert(b.Insts, s.ii, in)
	return s.where() + " duplicated"
}

func editSwapInsts(s *site) string {
	b := s.blk()
	if len(b.Insts) < 2 {
		return ""
	}
	j := s.r.Intn(len(b.Insts))
	if s.r.Intn(2) == 0 {
		j = min(s.ii+1, len(b.Insts)-1)
	}
	if j == s.ii {
		return ""
	}
	b.Insts[s.ii], b.Insts[j] = b.Insts[j], b.Insts[s.ii]
	return fmt.Sprintf("%s swapped with %d", s.where(), j)
}

func editInsert(s *site) string {
	b := s.blk()
	var in mir.Inst
	switch s.r.Intn(6) {
	case 0:
		in = mir.Inst{Op: mir.OpDrop, Args: []mir.LocalID{s.local()}}
	case 1:
		in = mir.Inst{Op: mir.OpRelease, Args: []mir.LocalID{s.local()}}
	case 2:
		in = mir.Inst{Op: mir.OpMove, Dst: s.local(), Args: []mir.LocalID{s.local()}}
	case 3:
		in = mir.Inst{Op: mir.OpCopy, Dst: s.local(), Args: []mir.LocalID{s.local()}}
	case 4:
		in = mir.Inst{Op: mir.OpUnit, Dst: s.local()}
	default:
		in = mir.Inst{Op: mir.OpNop}
	}
	b.Insts = slices.Insert(b.Insts, s.ii, in)
	return fmt.Sprintf("%s inserted %s", s.where(), in.Op)
}

func editArgDrop(s *site) string {
	in := s.inst()
	if len(in.Args) == 0 {
		return ""
	}
	k := s.r.Intn(len(in.Args))
	in.Args = slices.Delete(in.Args, k, k+1)
	return fmt.Sprintf("%s dropped argument %d of %s", s.where(), k, in.Op)
}

func editArgDup(s *site) string {
	in := s.inst()
	if len(in.Args) == 0 {
		return ""
	}
	k := s.r.Intn(len(in.Args))
	in.Args = slices.Insert(in.Args, k, in.Args[k])
	return fmt.Sprintf("%s duplicated argument %d of %s", s.where(), k, in.Op)
}

func editArgAdd(s *site) string {
	in := s.inst()
	in.Args = append(in.Args, s.local())
	return fmt.Sprintf("%s appended argument to %s", s.where(), in.Op)
}

func editArgSwap(s *site) string {
	in := s.inst()
	if len(in.Args) < 2 {
		return ""
	}
	i, j := s.r.Intn(len(in.Args)), s.r.Intn(len(in.Args))
	if i == j {
		return ""
	}
	in.Args[i], in.Args[j] = in.Args[j], in.Args[i]
	return fmt.Sprintf("%s swapped arguments %d and %d of %s", s.where(), i, j, in.Op)
}

func editInstType(s *site) string {
	in := s.inst()
	if s.r.Intn(3) == 0 {
		in.Type = 0
		return fmt.Sprintf("%s type of %s cleared", s.where(), in.Op)
	}
	in.Type = s.typ()
	return fmt.Sprintf("%s type of %s changed", s.where(), in.Op)
}

func editConstType(s *site) string {
	in := s.inst()
	if in.Op != mir.OpConst {
		return ""
	}
	in.Type = s.typ()
	return s.where() + " constant type changed"
}

func editEntity(s *site) string {
	in := s.inst()
	if s.r.Intn(4) == 0 {
		in.Ent = 0
		return fmt.Sprintf("%s entity of %s cleared", s.where(), in.Op)
	}
	kinds := []sem.EntityKind{sem.EntFn, sem.EntType, sem.EntVariant, sem.EntClosure, sem.EntStatic, sem.EntAlias}
	e, ok := s.ent(kinds[s.r.Intn(len(kinds))])
	if !ok {
		return ""
	}
	in.Ent = e
	return fmt.Sprintf("%s entity of %s replaced", s.where(), in.Op)
}

func editRedirectCall(s *site) string {
	in := s.inst()
	if in.Op != mir.OpCall && in.Op != mir.OpClosure && in.Op != mir.OpFnItem && in.Op != mir.OpBind {
		return ""
	}
	e, ok := s.ent(sem.EntFn)
	if !ok || e == in.Ent {
		return ""
	}
	in.Ent = e
	return fmt.Sprintf("%s %s redirected to %s", s.where(), in.Op, s.p.R.Entity(e).Name)
}

func editIndex(s *site) string {
	in := s.inst()
	switch s.r.Intn(4) {
	case 0:
		in.Index++
	case 1:
		in.Index--
	case 2:
		in.Index = s.r.Intn(8)
	default:
		in.Index = s.r.Intn(1 << 20)
	}
	return fmt.Sprintf("%s index of %s now %d", s.where(), in.Op, in.Index)
}

func editStr(s *site) string {
	in := s.inst()
	if len(s.m.strs) == 0 {
		return ""
	}
	in.Str = s.m.strs[s.r.Intn(len(s.m.strs))]
	return fmt.Sprintf("%s text of %s now %q", s.where(), in.Op, in.Str)
}

func editFlag(s *site) string {
	in := s.inst()
	flags := []struct {
		name string
		p    *bool
	}{{"share", &in.Share}, {"copy", &in.Copy}, {"async", &in.Async}, {"endscope", &in.EndScope}, {"scoped", &in.Scoped}, {"fixed", &in.Fixed}}
	f := flags[s.r.Intn(len(flags))]
	*f.p = !*f.p
	return fmt.Sprintf("%s flag %s of %s flipped", s.where(), f.name, in.Op)
}

func editPrim(s *site) string {
	in := s.inst()
	switch s.r.Intn(3) {
	case 0:
		if len(s.m.prims) == 0 {
			return ""
		}
		in.Prim = s.m.prims[s.r.Intn(len(s.m.prims))]
	case 1:
		if len(s.m.provs) == 0 {
			return ""
		}
		in.Prov = s.m.provs[s.r.Intn(len(s.m.provs))]
	default:
		in.DPrim = mir.DirectPrim(s.r.Intn(int(mir.DPPow) + 2))
	}
	return fmt.Sprintf("%s primitive tags of %s changed", s.where(), in.Op)
}

func editLiteral(s *site) string {
	in := s.inst()
	if in.Op != mir.OpConst {
		return ""
	}
	in.Lit.Kind = sem.LitKind(s.r.Intn(8))
	return s.where() + " literal kind changed"
}

func editLocalType(s *site) string {
	if len(s.f.Locals) == 0 {
		return ""
	}
	l := s.local()
	if int(l) >= len(s.f.Locals) {
		return ""
	}
	if s.r.Intn(2) == 0 {
		s.f.Locals[l].Type = s.f.Locals[s.local()%mir.LocalID(len(s.f.Locals))].Type
	} else {
		s.f.Locals[l].Type = s.typ()
	}
	return fmt.Sprintf("%s local %%%s retyped", s.f.Name, s.f.Locals[l].Name)
}

func editLocalFlag(s *site) string {
	if len(s.f.Locals) == 0 {
		return ""
	}
	l := &s.f.Locals[s.r.Intn(len(s.f.Locals))]
	flag := ""
	switch s.r.Intn(4) {
	case 0:
		l.Cell, flag = !l.Cell, "cell"
	case 1:
		l.Borrowed, flag = !l.Borrowed, "borrowed"
	case 2:
		l.Cross, flag = !l.Cross, "cross"
	default:
		flag = "named"
		if l.Ent != 0 {
			l.Ent = 0
		} else if e, ok := s.ent(sem.EntType); ok {
			l.Ent = e
		}
	}
	return fmt.Sprintf("%s local %%%s flag %s flipped", s.f.Name, l.Name, flag)
}

func editDirectFlag(s *site) string {
	if len(s.f.Locals) == 0 {
		return ""
	}
	if len(s.f.Direct) != len(s.f.Locals) {
		s.f.Direct = make([]bool, len(s.f.Locals))
	}
	l := s.r.Intn(len(s.f.Locals))
	s.f.Direct[l] = !s.f.Direct[l]
	return fmt.Sprintf("%s direct flag of %%%s flipped", s.f.Name, s.f.Locals[l].Name)
}

func editDirectSig(s *site) string {
	switch {
	case s.f.DSig == nil:
		s.f.DSig = &mir.DirectSig{Params: make([]sem.TypeID, len(s.f.Params)), Ret: s.typ()}
	case s.r.Intn(3) == 0:
		s.f.DSig = nil
	case s.r.Intn(2) == 0:
		s.f.DSig.Ret = s.typ()
	case len(s.f.DSig.Params) > 0:
		s.f.DSig.Params[s.r.Intn(len(s.f.DSig.Params))] = s.typ()
	default:
		return ""
	}
	return s.f.Name + " direct signature changed"
}

func editTermRemove(s *site) string {
	s.blk().Term = mir.Term{}
	return s.where() + " terminator removed"
}

func editTermTarget(s *site) string {
	t := &s.blk().Term
	if len(t.Targets) == 0 {
		return ""
	}
	t.Targets[s.r.Intn(len(t.Targets))] = s.block()
	return s.where() + " terminator retargeted"
}

func editTermSwapEdge(s *site) string {
	t := &s.blk().Term
	if len(t.Targets) != 2 {
		return ""
	}
	t.Targets[0], t.Targets[1] = t.Targets[1], t.Targets[0]
	return s.where() + " branch edges swapped"
}

func editTermKind(s *site) string {
	t := &s.blk().Term
	t.Op = mir.TermOp(s.r.Intn(5))
	return s.where() + " terminator kind changed"
}

func editTermOperand(s *site) string {
	t := &s.blk().Term
	if len(t.Args) == 0 {
		return ""
	}
	t.Args[0] = s.local()
	return s.where() + " terminator operand changed"
}

func editFunc(s *site) string {
	f := s.f
	switch s.r.Intn(8) {
	case 0:
		f.Ret = s.typ()
	case 1:
		if len(f.Params) == 0 {
			return ""
		}
		k := s.r.Intn(len(f.Params))
		f.Params = slices.Delete(f.Params, k, k+1)
	case 2:
		if len(f.Params) < 2 {
			return ""
		}
		f.Params[0], f.Params[len(f.Params)-1] = f.Params[len(f.Params)-1], f.Params[0]
	case 3:
		f.Params = append(f.Params, s.local())
	case 4:
		f.Async = !f.Async
	case 5:
		f.Suspends += s.r.Intn(3) - 1
	case 6:
		f.ImplicitOk = !f.ImplicitOk
	default:
		if len(f.Slot) == 0 {
			return ""
		}
		f.Slot[s.r.Intn(len(f.Slot))] = s.r.Intn(6) - 1
	}
	return f.Name + " function header changed"
}
