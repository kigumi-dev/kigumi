package sem

var (
	cStrayBangReturn = defineCode("E2184", "stray-bang-return", "`!` must touch the return type: write `-> {type}!`; a bare `!` right after the parameter list, with no `->`, means `-> Void!`")
)
