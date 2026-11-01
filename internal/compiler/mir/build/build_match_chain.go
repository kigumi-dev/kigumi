package build

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// andThen evaluates the next test only when the previous one succeeded.
func (b *builder) andThen(cond midir.LocalID, next func() midir.LocalID) midir.LocalID {
	result := b.temp(sem.TyBool)
	b.assign(result, cond)
	more, join := b.newBlockAt(), b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{cond}, Targets: []midir.BlockID{more, join}})
	b.cur = more
	b.assign(result, next())
	b.jump(join)
	b.cur = join
	return result
}

// orElse evaluates the next test only when the previous one failed.
func (b *builder) orElse(cond midir.LocalID, next func() midir.LocalID) midir.LocalID {
	result := b.temp(sem.TyBool)
	b.assign(result, cond)
	more, join := b.newBlockAt(), b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{cond}, Targets: []midir.BlockID{join, more}})
	b.cur = more
	b.assign(result, next())
	b.jump(join)
	b.cur = join
	return result
}
