package mir

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Dump renders the program for golden tests.
func (p *Program) Dump() string {
	var sb strings.Builder
	for _, f := range p.Funcs {
		p.dumpFunc(&sb, f)
	}
	return sb.String()
}

func (p *Program) dumpFunc(sb *strings.Builder, f *midir.Func) {
	if f.Async {
		sb.WriteString("async ")
	}
	fmt.Fprintf(sb, "fn %s(", f.Name)
	for i, prm := range f.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%%%s: %s", f.Locals[prm].Name, p.R.TypeString(f.Locals[prm].Type))
	}
	fmt.Fprintf(sb, ") -> %s ", p.R.TypeString(f.Ret))
	if f.Async && f.Slot != nil {
		fmt.Fprintf(sb, "[suspends %d, frame %d, %s] ", f.Suspends, f.Frame, p.mobility(f))
	}
	sb.WriteString("{\n")
	for bid, blk := range f.Blocks {
		fmt.Fprintf(sb, "  b%d:\n", bid)
		for _, in := range blk.Insts {
			sb.WriteString("    ")
			sb.WriteString(p.inst(f, in))
			sb.WriteByte('\n')
		}
		sb.WriteString("    ")
		sb.WriteString(p.term(f, blk.Term))
		sb.WriteByte('\n')
	}
	sb.WriteString("}\n")
}

func (p *Program) local(f *midir.Func, l midir.LocalID) string {
	if int(l) >= len(f.Locals) {
		return fmt.Sprintf("%%?%d", l)
	}
	return "%" + f.Locals[l].Name
}

func (p *Program) inst(f *midir.Func, in midir.Inst) string {
	var args []string
	for _, a := range in.Args {
		args = append(args, p.local(f, a))
	}
	head := ""
	switch in.Op {
	case midir.OpDrop, midir.OpSetField, midir.OpSetIndex, midir.OpCellSet, midir.OpGlobalSet, midir.OpPanic, midir.OpNop, midir.OpCancelDone:
	default:
		head = p.local(f, in.Dst) + " = "
	}
	body := in.Op.String()
	switch in.Op {
	case midir.OpConst:
		body += " " + litText(in) + ": " + p.R.TypeString(in.Type)
	case midir.OpCall, midir.OpVariant, midir.OpRecord, midir.OpClosure, midir.OpFnItem, midir.OpBind, midir.OpCFnPtr, midir.OpIsVariant, midir.OpIsType, midir.OpGlobalGet, midir.OpGlobalSet:
		body += " " + p.declName(in.Ent)
		if in.Str != "" {
			body += " " + in.Str
		}
		if in.Async {
			body += " async"
		}
	case midir.OpBinary, midir.OpUnary, midir.OpBuiltin:
		body += " " + in.Str
	case midir.OpField, midir.OpFieldMove, midir.OpPayload, midir.OpSetField, midir.OpBorrow:
		body += fmt.Sprintf(" %d", in.Index)
	case midir.OpBox:
		body += " " + p.R.TypeString(in.Type)
	case midir.OpShare:
		if in.Copy && in.IfWritable {
			body += " copy-mut"
		} else if in.Copy {
			body += " copy"
		} else {
			body += " retain"
		}
	case midir.OpBoxDirect:
		if in.Fixed {
			body += " fixed"
		}
	case midir.OpInterp, midir.OpShell:
		body += fmt.Sprintf(" %q", strings.Join(in.Strs, "\x00"))
	case midir.OpPanic:
		body += " " + fmt.Sprintf("%q", in.Str)
	}
	if len(args) > 0 {
		body += "(" + strings.Join(args, ", ") + ")"
	}
	return head + body
}

func litText(in midir.Inst) string {
	if in.Str != "" {
		return in.Str
	}
	l := in.Lit
	switch {
	case l.Int != nil:
		return l.Int.String()
	case l.Float != nil:
		return l.Float.Text('g', -1)
	case l.Kind == 3:
		return fmt.Sprintf("%q", l.Str)
	case l.Kind == 4:
		if l.Bool {
			return "true"
		}
		return "false"
	case l.Kind == 5:
		return fmt.Sprintf("%q", string(l.Char))
	case l.Kind == 6:
		return fmt.Sprintf("%q", l.Str)
	}
	return "0"
}

func (p *Program) term(f *midir.Func, t midir.Term) string {
	switch t.Op {
	case midir.TermJump:
		return fmt.Sprintf("jump b%d", t.Targets[0])
	case midir.TermBranch:
		return fmt.Sprintf("branch %s ? b%d : b%d", p.local(f, t.Args[0]), t.Targets[0], t.Targets[1])
	case midir.TermReturn:
		return "return " + p.local(f, t.Args[0])
	case midir.TermUnreachable:
		return "unreachable"
	}
	return "<no terminator>"
}

// mobility names how a Future's state may move, as classifyMobility left it.
func (p *Program) mobility(f *midir.Func) string {
	if e := p.R.Entity(f.Ent); e.Kind != sem.EntFn {
		return "unknown"
	}
	info := p.R.Fn(f.Ent)
	if info.BoundBy != "" {
		return info.Mobility.String() + " by " + info.BoundBy
	}
	if s := info.Mobility.String(); s != "" {
		return s
	}
	return "unknown"
}
