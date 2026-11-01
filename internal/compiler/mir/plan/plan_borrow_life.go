package plan

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// borrowLife tracks what a read keeps alive: a borrow keeps its source, and so does a temporary built from the borrow
// (record, variant, array, call result, projection, copy or a join of copies) whose type can still hold it.
type borrowLife struct {
	borrowed map[midir.LocalID]midir.LocalID
	carried  map[midir.LocalID][]midir.LocalID
	memo     map[midir.LocalID][]midir.LocalID
}

func (p *planner) newBorrowLife(f *midir.Func, isTemp func(midir.LocalID) bool, defs map[midir.LocalID]int, defBlocks map[midir.LocalID]map[int]bool, borrowed map[midir.LocalID]midir.LocalID) *borrowLife {
	carried := map[midir.LocalID][]midir.LocalID{}
	refs := map[sem.TypeID]bool{}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !carries(in.Op) || !definesTemp(in, isTemp) || defs[in.Dst] > 1 && in.Op != midir.OpCopy && in.Op != midir.OpMove || !p.mayCarry(f, in, refs) {
					continue
				}
				for _, a := range in.Args {
					_, alias := borrowed[a]
					if (alias || carried[a] != nil) && a != in.Dst && !slices.Contains(carried[in.Dst], a) {
						carried[in.Dst] = append(carried[in.Dst], a)
						changed = true
					}
				}
			}
		}
	}
	b := &borrowLife{borrowed: borrowed, carried: carried, memo: map[midir.LocalID][]midir.LocalID{}}
	b.pruneJoins(f, defs, defBlocks)
	return b
}

func carries(op midir.Opcode) bool {
	switch op {
	case midir.OpRecord, midir.OpVariant, midir.OpArray, midir.OpCall, midir.OpCallValue, midir.OpField, midir.OpPayload, midir.OpIndex, midir.OpCopy, midir.OpMove:
		return true
	}
	return false
}

// An aggregate holds what its operands hold, whatever its declared type (a variadic pack has the callee's Array[T]).
func (p *planner) mayCarry(f *midir.Func, in midir.Inst, refs map[sem.TypeID]bool) bool {
	if p.holdsRef(f.Locals[in.Dst].Type, refs) {
		return true
	}
	if in.Op != midir.OpRecord && in.Op != midir.OpVariant && in.Op != midir.OpArray {
		return false
	}
	return slices.ContainsFunc(in.Args, func(a midir.LocalID) bool { return p.holdsRef(f.Locals[a].Type, refs) })
}

func (b *borrowLife) root(l midir.LocalID) midir.LocalID {
	for {
		src, ok := b.borrowed[l]
		if !ok {
			return l
		}
		l = src
	}
}

// sources lists the borrowed temporaries that the borrows h carries end at.
func (b *borrowLife) sources(h midir.LocalID) []midir.LocalID {
	if out, ok := b.memo[h]; ok {
		return out
	}
	b.memo[h] = nil
	var out []midir.LocalID
	add := func(s midir.LocalID) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	for _, c := range b.carried[h] {
		for _, s := range b.through(c) {
			add(s)
		}
	}
	b.memo[h] = out
	return out
}

// through lists what a value carrying c keeps alive: the source c borrows, and what c carries.
func (b *borrowLife) through(c midir.LocalID) []midir.LocalID {
	if _, alias := b.borrowed[c]; alias {
		r := b.root(c)
		return append([]midir.LocalID{r}, b.sources(r)...)
	}
	return b.sources(c)
}

// keeps lists the temporaries a read of l keeps alive: the source its borrow chain ends at, and the sources that one carries.
func (b *borrowLife) keeps(l midir.LocalID) []midir.LocalID {
	r := b.root(l)
	return append([]midir.LocalID{r}, b.sources(r)...)
}

func (b *borrowLife) reads(args []midir.LocalID, t midir.LocalID) bool {
	for _, a := range args {
		if r := b.root(a); r == t || b.carried[r] != nil && slices.Contains(b.sources(r), t) {
			return true
		}
	}
	return false
}

// extend keeps a source alive while a borrow of it, or a value carrying that borrow, is read;
// such a read in another block sends the source to the liveness placement.
func (b *borrowLife) extend(last map[midir.LocalID]use, crossUse map[midir.LocalID]bool) {
	for changed := true; changed; {
		changed = false
		for alias, src := range b.borrowed {
			if extendTo(alias, src, last, crossUse) {
				changed = true
			}
		}
		for h := range b.carried {
			for _, s := range b.sources(h) {
				if crossUse[h] && !crossUse[s] {
					crossUse[s] = true
					changed = true
				}
				if extendTo(h, s, last, crossUse) {
					changed = true
				}
			}
		}
	}
}

func extendTo(from, to midir.LocalID, last map[midir.LocalID]use, crossUse map[midir.LocalID]bool) bool {
	au, ok := last[from]
	if !ok {
		return false
	}
	su := last[to]
	if au.block != su.block {
		if crossUse[to] {
			return false
		}
		crossUse[to] = true
		return true
	}
	if au.inst > su.inst {
		last[to] = au
		return true
	}
	return false
}

// holdsRef reports whether a value of type t can hold a reference: a reference, a type parameter (its instantiation may be one),
// or a type with such a type among its arguments.
func (p *planner) holdsRef(t sem.TypeID, memo map[sem.TypeID]bool) bool {
	if v, ok := memo[t]; ok {
		return v
	}
	memo[t] = false
	n := p.R.Types.Node(t)
	v := n.Kind == sem.KRef || n.Kind == sem.KParam || n.Elem != 0 && p.holdsRef(n.Elem, memo)
	for _, a := range n.Args {
		v = v || p.holdsRef(a, memo)
	}
	memo[t] = v
	return v
}
