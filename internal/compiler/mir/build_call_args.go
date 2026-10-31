package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// argFrame is a call's evaluated argument locals; off is 1 when the receiver leads them.
type argFrame struct {
	locals []midir.LocalID
	off    int
}

func (b *builder) evalArgs(off, n int, order []int, eval func(i int) midir.LocalID) []midir.LocalID {
	out := make([]midir.LocalID, n)
	b.argFrames = append(b.argFrames, argFrame{out, off})
	defer func() { b.argFrames = b.argFrames[:len(b.argFrames)-1] }()
	if order == nil {
		for i := 0; i < n; i++ {
			out[i] = eval(i)
		}
		return out
	}
	for _, i := range order {
		out[i] = eval(i)
	}
	return out
}

// argRef runs inside the thunk call's own frame, so the call it reads is the one below.
func (b *builder) argRef(index int) midir.LocalID {
	f := b.argFrames[len(b.argFrames)-2]
	if index < 0 {
		return f.locals[0]
	}
	return f.locals[f.off+index]
}

func (b *builder) valueSig(t sem.TypeID) sem.TypeID {
	tt := b.r.Types
	switch tt.Kind(t) {
	case sem.KClosure:
		return b.r.Closure(tt.Node(t).Ent).Sig
	case sem.KParam:
		for _, con := range b.r.TypeParam(tt.Node(t).Ent).Constraints {
			if con.Kind == sem.CFn || con.Kind == sem.CFnMut || con.Kind == sem.CFnOnce {
				return con.Type
			}
		}
	}
	return t
}

// packVariadic groups a variadic call's trailing arguments into one array, per sig's fixed arity.
func (b *builder) packVariadic(call *hir.Expr, out []midir.LocalID, sig sem.TypeID) []midir.LocalID {
	params := b.r.Types.Node(sig).Args
	fixed := len(params) - 1
	if fixed < 0 {
		fixed = 0
	}
	if fixed > len(out) {
		fixed = len(out)
	}
	arr := b.emit(midir.Inst{Op: midir.OpArray, Dst: b.temp(params[len(params)-1]), Args: append([]midir.LocalID{}, out[fixed:]...), Node: call.Node})
	return append(out[:fixed], arr)
}
