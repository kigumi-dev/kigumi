package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

// checkHandoff rejects an explicit cross-executor handoff of a future the
// compiler pinned to one executor; one it cannot trace is left to run time.
func checkHandoff(p *Program) {
	want, ok := opdef.StdOpIdOf("TASK_HANDOFF")
	if !ok {
		return
	}
	for _, f := range p.allFuncs() {
		t := newFutureTrace()
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				t.note(in)
				id, has := in.StdOp()
				if !has || id != want || len(in.Args) < 2 {
					continue
				}
				ent := t.asyncBody(in.Args[1])
				if ent == 0 || p.R.Fn(ent).Mobility != sem.MobilityExecutorBound {
					continue
				}
				emitHandoffDiag(p, f, in.Node, p.R.Fn(ent).BoundBy)
			}
		}
	}
}

// futureTrace follows a future local back to the async body it runs, across
// the alias and share temporaries the ownership plan inserts.
type futureTrace struct {
	item  map[midir.LocalID]sem.EntityID
	made  map[midir.LocalID]sem.EntityID
	alias map[midir.LocalID]midir.LocalID
}

func newFutureTrace() *futureTrace {
	return &futureTrace{item: map[midir.LocalID]sem.EntityID{}, made: map[midir.LocalID]sem.EntityID{}, alias: map[midir.LocalID]midir.LocalID{}}
}

func (t *futureTrace) note(in midir.Inst) {
	switch in.Op {
	case midir.OpFnItem:
		t.item[in.Dst] = in.Ent
	case midir.OpFutureNew:
		if len(in.Args) > 0 {
			t.made[in.Dst] = t.item[in.Args[0]]
		}
	case midir.OpAlias, midir.OpShare, midir.OpMove, midir.OpCopy:
		if len(in.Args) == 1 {
			t.alias[in.Dst] = in.Args[0]
		}
	}
}

func (t *futureTrace) asyncBody(l midir.LocalID) sem.EntityID {
	for i := 0; i < len(t.alias)+1; i++ {
		if ent, ok := t.made[l]; ok {
			return ent
		}
		next, ok := t.alias[l]
		if !ok {
			return 0
		}
		l = next
	}
	return 0
}

func (t *futureTrace) origin(l midir.LocalID) midir.LocalID {
	for i := 0; i < len(t.alias)+1; i++ {
		next, ok := t.alias[l]
		if !ok {
			break
		}
		l = next
	}
	return l
}

func emitHandoffDiag(p *Program, f *midir.Func, node syntax.NodeID, boundBy string) {
	c, ok := sem.CodeByName("handoff-executor-bound")
	if !ok {
		return
	}
	if node == 0 {
		node = p.R.Entity(f.Ent).Node
	}
	msg := sem.FillTemplate(c.Template, boundBy)
	if c.Help != "" {
		msg += "; " + c.Help
	}
	t := p.R.Tree(f.File)
	f.Diags = append(f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(node)), Msg: msg, Code: c.Num})
}
