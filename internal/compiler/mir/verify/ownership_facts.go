package verify

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

type ownershipFacts struct {
	*mir.Program
	unowned ownquery.LentMemo
}

func (p *ownershipFacts) unownedRefs(f *midir.Func) []bool { return p.unowned.Of(p.R, f) }
