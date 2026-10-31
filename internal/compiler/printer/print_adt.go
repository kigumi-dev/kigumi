package printer

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func (p *printer) variants(body syntax.NodeID) {
	vs := p.t.List(body)
	multi := vs.Len() > 3 || p.multiline(body)
	for i, vv := range vs.Enumerate() {
		switch {
		case multi:
			p.newline()
			p.indent++
			p.flushDocComments(vv.AsVariant().Name().Index())
			p.write("| ")
			p.indent--
		case i > 0:
			p.write(" | ")
		case vs.Len() == 1:
			// A lone variant keeps its bar: without it the declaration
			// reads back as an alias of that name.
			p.write(" | ")
		default:
			p.write(" ")
		}
		v := vv.AsVariant()
		p.write(v.Name().Text())
		if fields := v.Fields(); fields.Valid() {
			p.variantFields(v.Name().Index(), p.lastTok(vv.ID()), fields)
		}
		if multi {
			p.trailing(p.lastTok(vv.ID()), false)
		}
	}
	if multi {
		p.indent++
		p.endComments(p.lastTok(body))
		p.indent--
	}
}

// variantFields prints a payload; a line comment inside it puts each field on its own line so the comment stays beside its field.
func (p *printer) variantFields(name, last uint32, fields syntax.ListView) {
	open := name + 1
	p.write("(")
	if int(open) < len(p.t.Toks) && p.t.Toks[open].Kind == token.LParen && p.lineCommentIn(open+1, last) {
		p.newline()
		p.indent += 2
		for _, fv := range fields.Enumerate() {
			f := fv.AsVariantField()
			p.flushComments(p.firstTok(f.ID()))
			p.variantField(f)
			p.write(",")
			p.trailing(p.lastTok(f.ID()), true)
			p.newline()
		}
		p.flushComments(last)
		p.indent--
		p.write(")")
		p.indent--
		return
	}
	for j, fv := range fields.Enumerate() {
		if j > 0 {
			p.write(", ")
		}
		p.variantField(fv.AsVariantField())
	}
	p.write(")")
}

func (p *printer) variantField(f syntax.VariantFieldView) {
	if f.Flags()&syntax.FlagNamed != 0 {
		p.write(f.Name().Text() + ": ")
	}
	p.typ(f.Type().ID())
}

// endComments keeps the indented comment lines that follow the last variant inside the type; unindented ones belong to the next declaration.
func (p *printer) endComments(last uint32) {
	toks := p.t.Toks
	i := max(int(last)+1, p.cm.next)
	for i < len(toks) && toks[i].Kind == token.Newline && toks[i].Len() == 1 {
		i++
	}
	for i < len(toks) && toks[i].Kind == token.Comment && p.indentedComment(i) {
		p.newline()
		p.write(p.tok(uint32(i)))
		p.cm.next = max(p.cm.next, i+1)
		i++
		if i < len(toks) && toks[i].Kind == token.Newline && toks[i].Len() == 1 {
			i++
		}
	}
}

func (p *printer) indentedComment(i int) bool {
	start := p.t.Toks[i].Start
	if start == 0 {
		return false
	}
	c := p.t.File.Src[start-1]
	return c == ' ' || c == '\t'
}
