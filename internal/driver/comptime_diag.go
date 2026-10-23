package driver

import (
	"errors"
	"fmt"

	"kigumi/internal/diag"
	"kigumi/internal/mir"
	"kigumi/internal/sem"
	"kigumi/internal/token"
	"kigumi/internal/vm"
)

func comptimeDiag(res *sem.Result, ce vm.ComptimeError) diag.Diagnostic {
	tree := res.Tree(ce.Site.File)
	loc := diag.At(tree.File, tree.Span(ce.Site.Node))
	if ce.Err == nil {
		return diag.Warnf(loc, fmt.Sprintf("the `comptime` block printed %d bytes, which are discarded", ce.Printed)).
			WithHelp("return the text from the block and print it at run time")
	}
	var fail *vm.ComptimeFailure
	if !errors.As(ce.Err, &fail) {
		return diag.Errorf(loc, "comptime: "+ce.Err.Error())
	}
	name, args := "comptime-panic", []any{fail.Msg}
	if fail.Limit != "" {
		name, args = "comptime-limit", []any{fail.Limit}
	}
	code, _ := sem.CodeByName(name)
	d := diag.Errorf(loc, sem.FillTemplate(code.Template, args...))
	d.Code = code.Num
	if code.Help != "" {
		d = d.WithHelp(code.Help)
	}
	if at, ok := failureLine(res, fail.Site); ok {
		d = d.WithNote(at, "it failed at this line")
	}
	return d
}

func failureLine(res *sem.Result, site mir.Site) (diag.Location, bool) {
	if site.File == "" || site.Line == 0 {
		return diag.Location{}, false
	}
	for i := 1; i < len(res.Files); i++ {
		if f := res.Tree(sem.FileID(i)).File; f.Name == site.File && !f.Generated {
			return diag.At(f, trimLine(f, f.LineSpan(site.Line))), true
		}
	}
	return diag.Location{}, false
}

func trimLine(f *token.File, sp token.Span) token.Span {
	for sp.Start < sp.End && (f.Src[sp.Start] == ' ' || f.Src[sp.Start] == '\t') {
		sp.Start++
	}
	for sp.End > sp.Start && (f.Src[sp.End-1] == '\n' || f.Src[sp.End-1] == '\r') {
		sp.End--
	}
	return sp
}
