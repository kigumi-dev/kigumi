package verify

import "kigumi/internal/compiler/mir"

// claims checks a planned program against the allocation and interrupt claims its declarations make.
type claims struct{ *mir.Program }
