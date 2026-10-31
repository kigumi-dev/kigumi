package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

// exportShim defines the C symbol of an export(C) function whose records
// move by value, forwarding to kg_export_<name> (exportByval).
func (e *emitter) exportShim(f *midir.Func) {
	name := e.cName(f.Ent)
	sn := e.r.Types.Node(e.r.Fn(f.Ent).Sig)
	ret := e.cLang(sn.Elem)
	outRet := e.isCRecord(sn.Elem)
	var params, innerParams, innerArgs []string
	if outRet {
		innerParams, innerArgs = append(innerParams, ret+" *"), append(innerArgs, "&out")
	}
	for i, a := range sn.Args {
		p := fmt.Sprintf("a%d", i)
		params = append(params, e.cLang(a)+" "+p)
		if e.isCRecord(a) {
			innerParams, innerArgs = append(innerParams, "const "+e.cLang(a)+" *"), append(innerArgs, "&"+p)
		} else {
			innerParams, innerArgs = append(innerParams, e.cLang(a)), append(innerArgs, p)
		}
	}
	innerRet := ret
	if outRet {
		innerRet = "void"
	}
	call := fmt.Sprintf("kg_export_%s(%s)", name, strings.Join(innerArgs, ", "))
	body := "    return " + call + ";\n"
	switch {
	case outRet:
		body = fmt.Sprintf("    %s out;\n    %s;\n    return out;\n", ret, call)
	case ret == "void":
		body = "    " + call + ";\n"
	}
	fmt.Fprintf(&e.shim.fns, "extern %s kg_export_%s(%s);\n%s %s(%s) {\n%s}\n", innerRet, name, orVoid(innerParams), ret, name, orVoid(params), body)
}

// exportInterruptWrapper is export(interrupt)'s non-naked wrapper; sem (decl_interrupt.go) already forces Void-in, Void-out.
func (e *emitter) exportInterruptWrapper(f *midir.Func) string {
	return fmt.Sprintf("define void @%s() {\nentry:\n  call ptr %s()\n  ret void\n}\n\n", e.cName(f.Ent), e.sym(f))
}

func orVoid(parts []string) string {
	if len(parts) == 0 {
		return "void"
	}
	return strings.Join(parts, ", ")
}

func sanitize(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
