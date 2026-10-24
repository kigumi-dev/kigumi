package driver_test

// Each recipe line is an edit, the function, block and instruction it landed on, the opcode found there, and its choices; no rule means the back ends must take the mutant.
var pinnedMutants = []struct{ base, rule, recipe string }{
	{"adt", "cellops", `
opcode-any "main.Point.bump" b0/0 cellget | i32
`},
	{"adt", "cellops", `
opcode-arity "main.Point.bump" b0/0 cellget | i11
`},
	{"direct_fixed_array", "calls", `
opcode-any "main.main.closure2" b0/4 release | i44
`},
	{"iface_static", "calls", `
opcode-any "main.Pair.show" b0/10 release | i44
`},
	{"direct_aggregate", "stdop", `
redirect-call "main.firstPoint" b0/1 call | efn:"std/json||put"
`},
	{"mut_self_dispatch", "", `
func "main.Shape.grow" b0/1 isvariant | i6
`},
	{"iface_static", "ownership", `
insert "main.Pair.parse" b6/1 alias | i0 l"x"
`},
	{"generic_direct_methods", "droplent", `
operand "main.Point.choose[main.Point]" b1/0 move | i0 l"self"
`},
	{"generics", "", `
entity "main" b11/1 istype | i1 i5 ealias:"||byte"
`},
	{"direct_fixed_array", "structure", `
local-flag "std/math.min#(i8,i8)" b0/0 bin | i0 i0
func "main.Point.hash" b0/0 builtin | i3 l+0
`},
	{"direct_closure", "types", `
term-operand "main.lendsCapture" b0/0 alias | l"t2"
`},
	{"direct_closure", "types", `
entity "main.main.closure3" b2/2 alias | i0
term-operand "main.lendsCapture" b0/1 share | l"t1"
`},
	{"iface_static", "refs", `
duplicate "main.Pair.show" b0/10 release
`},
	{"direct_adt", "structure", `
term-kind "std/math.clamp#(u32,u32,u32)" b1/1 share | i0
type "main.Point.equals" b0/0 builtin | i0
`},
	{"mut_self_dispatch", "cellops", `
operand "main.growGeneric" b0/18 callv | i1 l"t4"
`},
	{"generics", "refs", `
insert "main.tally" b7/21 drop | i0 l"t5"
`},
	{"async", "types", `
operand "main.total" b4/1 bin | i0 l"t10"
`},
	{"async_method", "", `
operand "main.root" b4/3 release | i0 l"y"
`},
	{"iface_static", "stdop", `
prim "main.showAll" b0/1 share | i1 v"HOST_STDERR"
type "main.parseOne" b0/2 share | i1 t"&mut Map[K, V]"#3
`},
	{"conform_generic_method_dispatch", "types", `
local-type "std/io.BufReader.new" b0/0 move | l"t1" i0 l"t3"
`},
	{"async_method", "defs", `
duplicate "main" b0/13 unit
remove "main" b0/3 future
`},
	{"mut_self_dispatch", "calls", `
arg-swap "main.bumpForwarded" b0/6 call | i3 i0
`},
	{"iface_static", "shape", `
flag "main.showAll" b0/6 drop | i2
`},
	{"direct_closure", "refs", `
insert "main.peek" b0/0 field | i0 l"p"
`},
	{"mut_self_dispatch", "cellops", `
operand "main.Shape.flattenViaClosure.closure1" b0/1 cellset | i1 l"self"
`},
	{"generics", "calls", `
opcode-any "main.show" b0/0 alias | i5
`},
	{"generics", "refs", `
insert "main.tally" b7/16 share | i1 l"counts"
index "std/net.soSndTimeo" b0/0 const | i2 i1
`},
	{"direct_closure", "shape", `
opcode-any "main.aggregates.closure1" b0/7 drop | i35
`},
	{"generic_direct_methods", "defs", `
remove "main.Tag.equals" b0/0 builtin
`},
	{"generic_direct_methods", "refs", `
operand "main.Point.choose[main.Point]" b1/1 alias | i0 l"b"
`},
	{"direct_closure", "aggregates", `
operand "main.peek" b0/0 field | i0 l"t1"
`},
	{"mut_self_dispatch", "calls", `
operand "main.bumpForwarded" b0/6 call | i0 l"t6"
`},
	{"mut_self_dispatch", "cellops", `
operand "main.bumpBorrowed" b0/6 callv | i1 l"t4"
`},
	{"direct_closure", "entities", `
entity "main.lendsCapture.closure1" b0/1 call | i2 i3 eclosure:"main|main|closure"
`},
	{"iface_static", "aggregates", `
opcode-any "main.Pair.show" b0/2 callv | i15
`},
	{"generics", "aggregates", `
opcode-any "main" b2/1 alias | i14
`},
	{"adt", "refs", `
local-flag "main" b14/3 alias | i25 i3
`},
	{"adt", "refs", `
local-flag "main" b2/6 interp | i25 i3
`},
	{"mut_self_dispatch", "shape", `
opcode-arity "main" b0/219 release | i3
`},
	{"iface_static", "calls", `
arg-drop "main" b4/25 call | i0
`},
	{"mut_self_dispatch", "cellops", `
opcode-any "main.Tally.bump" b0/10 release | i3
`},
	{"generics", "refs", `
operand "main.tally" b3/3 drop | i0 l"t5"
`},
	{"async_method", "", `
local-flag "main" b0/10 release | i0 i2
`},
	{"async_method", "entry", `
remove "main" b0/1 alias
func "main" b0/3 release | i3 l"ex"
`},
	{"cfnptr", "calls", `
local-type "main.applyTwice" b0/4 drop | l"t3" i0 l"f"
`},
	{"cfnptr", "calls", `
type "main.applyTwice" b0/2 callc | i1 t"Ordering!"
`},
	{"cfnptr", "builtins", `
operand "main" b0/15 builtin | i0 l"t7"
`},
	{"direct_fixed_array", "builtins", `
text "main.Point.hash" b0/0 builtin | s"panic"
`},
}
