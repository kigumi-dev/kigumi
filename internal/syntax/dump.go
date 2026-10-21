package syntax

import (
	"fmt"
	"strings"

	"kigumi/internal/token"
)

// Dump renders the tree as indented text; the parser golden tests compare it.
func (t *Tree) Dump() string {
	d := &dumper{}
	t.dump(d, t.Root, "")
	return d.sb.String()
}

// dumper reuses one indent buffer instead of rebuilding it for every line.
type dumper struct {
	sb  strings.Builder
	ind []byte
}

func (d *dumper) push() { d.ind = append(d.ind, ' ', ' ') }
func (d *dumper) pop()  { d.ind = d.ind[:len(d.ind)-2] }

func (t *Tree) dump(d *dumper, id NodeID, label string) {
	d.sb.Write(d.ind)
	if label != "" {
		fmt.Fprintf(&d.sb, "%s: ", label)
	}
	if id == 0 {
		d.sb.WriteString("-\n")
		return
	}
	n := t.nodes[id]
	d.sb.WriteString(n.Kind.String())
	switch n.Kind {
	case Ident, IntLit, FloatLit, CharLit, BoolLit, StringLit, ByteStringLit, MemberExpr,
		OptMemberExpr, PatBind, PatWildcard, FieldInit, NamedArg, PatField,
		Variant, Param, InstantiateDecl, Unary, Binary, AssignStmt,
		Field, PatRange, TypeLifetime, UGRule, UGRequirement:
		fmt.Fprintf(&d.sb, " %s", t.TokText(n.Tok))
	case GenericParam:
		fmt.Fprintf(&d.sb, " %s", t.TokText(n.Tok))
		switch {
		case n.Lhs&FlagConst != 0:
			d.sb.WriteString(" const")
		case n.Lhs&FlagLifetime != 0:
			d.sb.WriteString(" lifetime")
		}
	case ShellLit:
		fmt.Fprintf(&d.sb, " %s", t.TokText(n.Lhs))
	case TestDecl:
		fmt.Fprintf(&d.sb, " %s", t.TokText(n.Lhs))
	case Visibility:
		if n.Lhs != 0 {
			fmt.Fprintf(&d.sb, " %s", t.TokText(n.Lhs))
		}
	case AbiBlock, AsmAbi:
		fmt.Fprintf(&d.sb, " %s", t.TokText(n.Tok))
	case ContractExpr, ContractBlock:
		fmt.Fprintf(&d.sb, " %s", modsString(n.Lhs))
	case VariantField:
		fmt.Fprintf(&d.sb, " %s", t.TokText(n.Tok))
		if n.Lhs&FlagNamed != 0 {
			d.sb.WriteString(" named")
		}
	case TypeRef:
		if t.Toks[n.Tok].Kind == token.Lifetime {
			fmt.Fprintf(&d.sb, " %s", t.TokText(n.Tok))
		}
		if n.Lhs&FlagMut != 0 {
			d.sb.WriteString(" mut")
		}
	case BorrowExpr, TypePtr:
		if n.Lhs&FlagMut != 0 {
			d.sb.WriteString(" mut")
		}
	case Path:
		d.sb.WriteString(" ")
		for i, tk := range t.extra[n.Lhs:n.Rhs] {
			if i > 0 {
				d.sb.WriteString(".")
			}
			d.sb.WriteString(t.TokText(tk))
		}
	case ShellText:
		fmt.Fprintf(&d.sb, " %q", t.File.Src[n.Lhs:n.Rhs])
	case ShellRedirect:
		r := t.View(id).AsShellRedirect()
		fmt.Fprintf(&d.sb, " %s fd=%d", redirNames[r.Op()], r.Fd())
	}
	d.sb.WriteString("\n")
	t.dumpChildren(d, id)
}

func (t *Tree) dumpChildren(d *dumper, id NodeID) {
	n := t.nodes[id]
	d.push()
	defer d.pop()
	child := func(c uint32) {
		if c != 0 || n.Kind == Binary || n.Kind == Unary || n.Kind == AssignStmt {
			t.dump(d, NodeID(c), "")
		}
	}
	switch kindTable[n.Kind].layout {
	case layL:
		child(n.Lhs)
	case layLR:
		child(n.Lhs)
		child(n.Rhs)
	case layFlagR, layTokR:
		child(n.Rhs)
	case layList:
		for _, c := range t.extra[n.Lhs:n.Rhs] {
			t.dump(d, NodeID(c), "")
		}
	case layRec:
		for i, s := range kindSchemas[n.Kind].slots {
			t.dumpSlot(d, n.Kind, s, t.extra[n.Lhs+uint32(i)])
		}
	}
}

func (t *Tree) dumpSlot(d *dumper, kind NodeKind, s slot, v uint32) {
	name := strings.ToLower(s.name[:1]) + s.name[1:]
	if name == "dir" {
		name = "kind"
	}
	switch s.role {
	case roleInt:
		if v == 0 || kind == ShellRedirect || kind == Param && name == "thunk" {
			return
		}
		text := fmt.Sprint(v)
		switch name {
		case "mods":
			text = modsString(v)
		case "flags":
			text = flagsString(v)
		}
		d.sb.Write(d.ind)
		fmt.Fprintf(&d.sb, "%s: %s\n", name, text)
	case roleTok:
		if v == 0 {
			return
		}
		text := t.TokText(v)
		if t.Toks[v].Kind == token.DocComment {
			text = fmt.Sprintf("%q", text)
		}
		d.sb.Write(d.ind)
		fmt.Fprintf(&d.sb, "%s: %s\n", name, text)
	default:
		if v == 0 && kind != FnDecl {
			return
		}
		t.dump(d, NodeID(v), name)
	}
}

var redirNames = [...]string{"?", "<", ">", ">>", "<&", ">&"}

func modsString(m uint32) string {
	var parts []string
	switch {
	case m&ModPureVar != 0:
		parts = append(parts, "pure?")
	case m&ModPure != 0:
		parts = append(parts, "pure")
	}
	for _, e := range []struct {
		bit  uint32
		name string
	}{{ModNoalloc, "noalloc"}, {ModAsync, "async"}, {ModUnsafe, "unsafe"}, {ModBareErr, "!"}, {ModStrayBang, "stray!"}} {
		if m&e.bit != 0 {
			parts = append(parts, e.name)
		}
	}
	return strings.Join(parts, " ")
}

func flagsString(f uint32) string {
	var parts []string
	for _, e := range []struct {
		bit  uint32
		name string
	}{{FlagMut, "mut"}, {FlagMove, "move"}, {FlagSelf, "self"}, {FlagVariadic, "variadic"}, {FlagEscaping, "escaping"}} {
		if f&e.bit != 0 {
			parts = append(parts, e.name)
		}
	}
	for i := range MaxEscapingParams {
		if f&(FlagEscapingParam0<<i) != 0 {
			parts = append(parts, fmt.Sprintf("escaping#%d", i))
		}
	}
	return strings.Join(parts, " ")
}
