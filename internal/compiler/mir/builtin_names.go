package mir

import midir "kigumi/internal/compiler/mir/ir"

const (
	BuiltinDirectEq   = "direct.eq"
	BuiltinDirectHash = "direct.hash"
)

func ProtoBuiltin(in midir.Inst) bool {
	return in.Op == midir.OpBuiltin && (in.Str == BuiltinDirectEq || in.Str == BuiltinDirectHash)
}

const (
	BuiltinCmpUnordered = "cmp.unordered"
	BuiltinCmpGuard     = "cmp.guard"
)
