package printer

import "kigumi/internal/syntax"

func (p *printer) fields(fields syntax.ListView) {
	_, closing := p.bracePairAfter(p.cm.next)
	if fields.Len() == 0 {
		p.commentsOrEmpty(closing)
		return
	}
	p.write("{")
	p.newline()
	p.indent++
	for i, fv := range fields.Enumerate() {
		f := fv.AsField()
		if i > 0 && (f.Metadata().Valid() || fields.At(i-1).AsField().Metadata().Valid()) {
			p.blank()
		}
		p.flushComments(p.firstTok(f.ID()))
		p.docLines(p.t.View(f.ID()).AsDecl())
		if vis := f.Vis(); vis.Valid() {
			p.visibility(vis)
		}
		if f.Flags()&syntax.FlagMut != 0 {
			p.write("mut ")
		}
		p.write(f.Name().Text() + " ")
		p.typ(f.Type().ID())
		if meta := f.Metadata(); meta.Valid() {
			p.write(" ")
			p.metadata(meta)
		}
		p.trailing(p.lastTok(f.ID()), true)
		p.newline()
	}
	p.flushComments(closing)
	p.indent--
	p.write("}")
}

// commentsOrEmpty prints `{}` for a body with nothing in it, or the comments alone when it holds some.
func (p *printer) commentsOrEmpty(closing uint32) {
	if !p.hasCommentBefore(closing) {
		p.write("{}")
		return
	}
	p.write("{")
	p.newline()
	p.indent++
	p.flushComments(closing)
	p.indent--
	p.write("}")
}

func (p *printer) metadata(meta syntax.MetadataView) {
	p.write("{")
	p.newline()
	p.indent++
	for iv := range meta.Views() {
		item := iv.AsMetaItem()
		p.write(item.Path().Join("."))
		if args := item.Args(); args.Valid() {
			p.write("(")
			p.exprList(args.ID())
			p.write(")")
		}
		p.newline()
	}
	p.indent--
	p.write("}")
}
