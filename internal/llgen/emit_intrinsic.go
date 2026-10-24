package llgen

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// intrinsic declares @llvm.<name> once for the module and returns its symbol.
func (e *emitter) intrinsic(name, ret string, params ...string) string {
	if e.intrinsics == nil {
		e.intrinsics = map[string]string{}
	}
	e.intrinsics[name] = fmt.Sprintf("declare %s @llvm.%s(%s)\n", ret, name, strings.Join(params, ", "))
	return "@llvm." + name
}

func (e *emitter) intrinsicDecls() string {
	var sb strings.Builder
	for _, name := range slices.Sorted(maps.Keys(e.intrinsics)) {
		sb.WriteString(e.intrinsics[name])
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String()
}

func (e *emitter) withOverflow(sb *strings.Builder, op, ty, l, r string) (string, string) {
	pair := fmt.Sprintf("{ %s, i1 }", ty)
	fn := e.intrinsic(op+".with.overflow."+ty, pair, ty, ty)
	call, v, o := e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = call %s %s(%s %s, %s %s)\n", call, pair, fn, ty, l, ty, r)
	fmt.Fprintf(sb, "  %s = extractvalue %s %s, 0\n  %s = extractvalue %s %s, 1\n", v, pair, call, o, pair, call)
	return v, o
}

func (e *emitter) callIntrinsic(sb *strings.Builder, name, ty string, args ...string) string {
	params := make([]string, len(args))
	typed := make([]string, len(args))
	for i, a := range args {
		params[i] = ty
		typed[i] = ty + " " + a
	}
	fn := e.intrinsic(name, ty, params...)
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", t, ty, fn, strings.Join(typed, ", "))
	return t
}
