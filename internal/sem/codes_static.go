package sem

// `static`/`static mut`, static-origin borrows and
// foreign data symbols.
var (
	cStaticNoInit         = defineCode("E1190", "static-no-initializer", "`{name}` needs a compile-time initializer; write `= <expr>`")
	cStaticExternInit     = defineCode("E1191", "static-extern-initializer", "an `extern` static cannot have an initializer; its storage is provided by the external definition")
	cStaticNotInitable    = defineCode("E1192", "static-not-initializable", "`{type}` cannot be a static's type; a static must be an allocation-free, drop-free `Copy` value; move the resource or owning value behind an explicit accessor instead")
	cStaticAssign         = defineCode("E1193", "static-assign", "`{name}` is an immutable static; direct assignment is not allowed; use `static mut` and an `unsafe` block, or a package-private wrapper function")
	cStaticBorrowMut      = defineCode("E1194", "static-borrow-mut", "`&mut {name}` cannot be taken of a static place; use `static mut` with an unsafe raw pointer")
	cStaticMoveOut        = defineCode("E1195", "static-move-out", "`{name}` is a static place and its type is not `Copy`; a non-Copy value cannot move out of a static")
	cStaticMutOrdinaryRef = defineCode("E1197", "static-mut-ordinary-ref", "an ordinary `&{name}`/`&mut {name}` cannot be taken of a `static mut`, even inside `unsafe`; use an explicit raw pointer operation instead")
	cStaticBorrowEscape   = defineCode("E1198", "static-borrow-escape", "a static-origin borrow cannot escape its call/local scope in v1; return an owned copy, expose an operation/callback, or use an unsafe raw pointer with an explicit contract")
	cStaticLifetime       = defineCode("E1199", "static-lifetime-reserved", "program-lifetime references are not supported in v1; use the static directly, pass a call-scoped borrow, or an unsafe raw pointer with an explicit contract")
	cStaticExternAbiType  = defineCode("E1201", "extern-static-abi-type", "`{type}` is not a C ABI-safe type for an `extern`/`export` static")
	cStaticExternWrite    = defineCode("E1203", "extern-static-immutable-write", "`{name}` is declared `extern(C) static` without `mut`; the binding contract says the external definition never changes; declare it `static mut` if the foreign definition can change")
	cLinkNameBadTarget    = defineCode("E1204", "link-name-bad-target", "`@linkName` is only allowed on a function or static inside an `extern`/`export` block")
	cLinkNameBadArg       = defineCode("E1205", "link-name-bad-arg", "`@linkName` takes exactly one nonempty string literal made of letters, digits, `_`, `.` and `$`")
	cSectionBadTarget     = defineCode("E1206", "section-bad-target", "`@section` is only allowed on a Kigumi-owned function or static definition, not an `extern` declaration")
	cSectionBadArg        = defineCode("E1207", "section-bad-arg", "`@section` takes exactly one nonempty string literal")
	cAttrDuplicate        = defineCode("E1208", "attr-duplicate", "`@{name}` cannot be given twice on the same declaration")
	cLinkNameDuplicate    = defineCode("E1209", "link-name-duplicate-symbol", "the external name `{name}` is already used by another exported declaration")
)
