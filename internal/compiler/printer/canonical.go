package printer

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

// Print renders the tree in canonical form. It is only defined
// for trees without diagnostics; callers must check Tree.Diags first.
func Print(t *syntax.Tree) string {
	pr := &printer{t: t, cm: newComments(t)}
	pr.file(t.Root)
	pr.flushComments(uint32(len(t.Toks)))
	return pr.sb.String()
}

type printer struct {
	t      *syntax.Tree
	sb     strings.Builder
	indent int
	// indentBuf is grown (never shrunk) and resliced to the current indent's
	// width, instead of building a fresh strings.Repeat for every line.
	indentBuf []byte
	cm        *comments
	// lineStart is true when nothing has been written on the current line yet.
	lineStart bool
	// chain is set once a statement has broken a method chain onto new lines,
	// so later `.` segments do not indent again.
	chain  bool
	eofIdx int
}

// withIndent runs f and restores the indent (and chain state) afterwards, so
// a multi-line method chain inside f cannot leak its extra indent.
func (p *printer) withIndent(f func()) {
	indent, chain := p.indent, p.chain
	f()
	p.indent, p.chain = indent, chain
}

func (p *printer) write(s string) {
	if p.lineStart && s != "" {
		p.writeIndent()
		p.lineStart = false
	}
	p.sb.WriteString(s)
}

func (p *printer) writeIndent() {
	need := p.indent * 4
	for len(p.indentBuf) < need {
		p.indentBuf = append(p.indentBuf, ' ')
	}
	p.sb.Write(p.indentBuf[:need])
}

func (p *printer) newline() {
	p.sb.WriteString("\n")
	p.lineStart = true
}

func (p *printer) blank() {
	if !strings.HasSuffix(p.sb.String(), "\n\n") && p.sb.Len() > 0 {
		p.newline()
	}
}

func (p *printer) tok(idx uint32) string { return p.t.TokText(idx) }

func (p *printer) file(root syntax.NodeID) {
	decls, tight, run := p.sortImports(p.t.List(root).AppendIDs(nil))
	for i, d := range decls {
		if i > 0 && !tight[d] {
			p.separateDecls(decls[i-1], d)
		}
		var tv *importTrivia
		if run != nil {
			tv = run.byDecl[d]
			if i == run.first {
				p.flushComments(run.header)
				if run.headerBlank {
					p.blank()
				}
			}
		}
		if tv != nil {
			p.importComments(tv)
		} else {
			p.flushComments(p.firstTok(d))
		}
		p.withIndent(func() { p.decl(d) })
		if tv != nil {
			p.importLineEnd(tv)
		} else {
			p.endLine(d)
		}
	}
}

func (p *printer) multiline(id syntax.NodeID) bool {
	sp := p.t.Span(id)
	return p.t.File.Line(sp.Start) != p.t.File.Line(sp.End-1)
}

func (p *printer) decl(id syntax.NodeID) {
	switch p.t.Kind(id) {
	case syntax.FnDecl:
		p.fnDecl(id)
	case syntax.TypeDecl:
		p.typeDecl(id)
	case syntax.InterfaceDecl:
		p.interfaceDecl(id)
	case syntax.ConstDecl:
		p.constDecl(id)
	case syntax.StaticDecl:
		p.staticDecl(id)
	case syntax.ContextKeyDecl:
		p.contextKeyDecl(id)
	case syntax.ImportDecl:
		p.importDecl(id)
	case syntax.TestDecl:
		d := p.t.View(id).AsTestDecl()
		p.write("test " + d.Name().Text() + " ")
		p.block(d.Body().ID())
	case syntax.AbiBlock:
		p.abiBlock(id)
	case syntax.ContractBlock:
		d := p.t.View(id).AsContractBlock()
		p.write(modsText(d.Mods()) + " ")
		p.declList(d.Body().ID())
	case syntax.InstantiateDecl:
		d := p.t.View(id).AsInstantiateDecl()
		p.write("instantiate " + d.Name().Text() + " = ")
		p.typ(d.Type().ID())
	default:
		p.stmt(id)
	}
}

func (p *printer) declList(list syntax.NodeID) {
	items := p.t.List(list)
	_, closing := p.bracePairAfter(p.cm.next)
	if items.Len() == 0 {
		p.commentsOrEmpty(closing)
		return
	}
	p.write("{")
	p.newline()
	p.indent++
	for c := items.Cursor(); c.Next(); {
		d := c.ID()
		if c.Index() > 0 {
			p.separateDecls(items.ItemID(c.Index()-1), d)
		}
		p.flushComments(p.firstTok(d))
		p.withIndent(func() { p.decl(d) })
		p.endLine(d)
	}
	p.flushComments(p.closeOfNext(p.lastTok(items.ItemID(items.Len() - 1))))
	p.indent--
	p.write("}")
}

func modsText(m uint32) string {
	var parts []string
	switch {
	case m&syntax.ModPureVar != 0:
		parts = append(parts, "pure?")
	case m&syntax.ModPure != 0:
		parts = append(parts, "pure")
	}
	for _, e := range []struct {
		bit  uint32
		name string
	}{{syntax.ModNoalloc, "noalloc"}, {syntax.ModAsync, "async"}, {syntax.ModUnsafe, "unsafe"}} {
		if m&e.bit != 0 {
			parts = append(parts, e.name)
		}
	}
	return strings.Join(parts, " ")
}
