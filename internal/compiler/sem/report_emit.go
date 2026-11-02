package sem

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

func (r *Result) noteDecl(d diag.Diagnostic, b Binding, msg string) diag.Diagnostic {
	ent := b.Ent
	if ent == 0 && b.Set != 0 && len(r.Overloads[b.Set].Members) > 0 {
		ent = r.Overloads[b.Set].Members[0]
	}
	if ent == 0 {
		return d
	}
	e := r.Entity(ent)
	if e.File == 0 || e.Tok == 0 {
		return d
	}
	return d.WithNote(r.loc(e.File, r.Files[e.File].Tree.Toks[e.Tok].Span), msg)
}

func (r *Result) errNote(f FileID, n syntax.NodeID, prev Binding, note string, c Code, args ...any) bool {
	d, ok := r.diagAt(f, n, c, args...)
	if ok {
		r.emitDiag(f, r.noteDecl(d, prev, note))
	}
	return ok
}

// a poison argument means the error was already reported at its origin.
func (r *Result) suppressed(args []any) bool {
	for _, a := range args {
		switch v := a.(type) {
		case TypeID:
			if v == TyPoison || v == NoType {
				return true
			}
		case EntityID:
			if r.poisoned(v) {
				return true
			}
		}
	}
	return false
}

func (r *Result) fmtArg(a any) string {
	switch v := a.(type) {
	case TypeID:
		return r.TypeString(v)
	case EntityID:
		return r.entityName(v)
	case string:
		return v
	}
	return fmt.Sprint(a)
}

func (r *Result) fill(template string, args []any) string {
	return fillTemplate(template, r.fmtArg, args)
}

// FillTemplate is fillTemplate for a caller with no *Result (internal/toolchain/driver builds E994's
// message before a sem.Result exists); args must be a string or an int.
func FillTemplate(template string, args ...any) string {
	return fillTemplate(template, func(a any) string { return fmt.Sprint(a) }, args)
}

// FillCode is FillTemplate applied to a whole Code: message holes consume
// args first, the same way diagSpan splits them, and help takes the rest.
func FillCode(c Code, args ...any) (msg, help string) {
	return fillCode(c, func(a any) string { return fmt.Sprint(a) }, args)
}

// Plural is plural, exported for the same reason as FillTemplate.
func Plural(n int) string { return plural(n) }

func (r *Result) diagAt(f FileID, n syntax.NodeID, c Code, args ...any) (diag.Diagnostic, bool) {
	if f == 0 || r.suppressed(args) || r.afterFailedImport(f, c, args) {
		return diag.Diagnostic{}, false
	}
	return r.diagSpan(f, r.Files[f].span(n), c, args...)
}

func (r *Result) diagSpan(f FileID, span token.Span, c Code, args ...any) (diag.Diagnostic, bool) {
	msg, help := fillCode(c, r.fmtArg, args)
	key := diagKey{f, span.Start, c.Name, msg}
	if _, dup := r.seen[key]; dup {
		return diag.Diagnostic{}, false
	}
	r.seen[key] = struct{}{}
	d := diag.Diagnostic{Severity: diag.Error, Loc: r.loc(f, span), Msg: msg, Code: c.Num}
	if c.Help != "" {
		d.Help = append(d.Help, help)
	}
	if strings.HasPrefix(c.Num, "W") {
		d.Severity = diag.Warning
	}
	return d, true
}

func (r *Result) emitDiag(f FileID, d diag.Diagnostic) {
	r.Files[f].Diags = append(r.Files[f].Diags, d)
}

func (r *Result) errAt(f FileID, n syntax.NodeID, c Code, args ...any) bool {
	d, ok := r.diagAt(f, n, c, args...)
	if ok {
		r.emitDiag(f, d)
	}
	return ok
}

func (r *Result) errFix(f FileID, n syntax.NodeID, fix diag.Fix, c Code, args ...any) bool {
	d, ok := r.diagAt(f, n, c, args...)
	if ok {
		d.Fixes = append(d.Fixes, fix)
		r.emitDiag(f, d)
	}
	return ok
}

// hasErrorAt reports an error already emitted on exactly n's span, so a follow-up check stays quiet.
func (r *Result) hasErrorAt(f FileID, n syntax.NodeID) bool {
	span := r.Files[f].span(n)
	for _, d := range r.Files[f].Diags {
		if d.Severity == diag.Error && d.Loc.Span == span {
			return true
		}
	}
	return false
}
