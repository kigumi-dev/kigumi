package syntax

import "slices"

type layout uint8

const (
	layLeaf  layout = iota // Tok only
	layL                   // Lhs is a child
	layLR                  // Lhs and Rhs are children
	layList                // Extra[Lhs:Rhs] are children
	layRec                 // Extra[Lhs:Rhs] are the declared slots in order
	layPath                // Extra[Lhs:Rhs] are token indices
	layFlagR               // Lhs is a flag word, Rhs a child
	layTokR                // Lhs is a token index, Rhs a child
	laySpan                // Lhs and Rhs are byte offsets
)

type role uint8

const (
	roleChild role = iota // a node id
	roleList              // a List node whose items are of kinds
	roleTok               // a token index
	roleInt               // a flag word or small integer
	roleItems             // the node's own Extra range, items of kinds
	roleToks              // the node's own Extra range of token indices
)

// slot is one named storage position: Lhs then Rhs for the fixed layouts, the
// Extra entries in order for layRec.
type slot struct {
	name  string
	role  role
	kinds kindSet
	opt   bool
}

// kindSchema declares one node kind; tok names the accessor for the node's own token.
type kindSchema struct {
	kind   NodeKind
	view   string
	layout layout
	tok    string
	slots  []slot
}

type kindSet [2]uint64

func set(ks ...NodeKind) kindSet {
	var s kindSet
	for _, k := range ks {
		s[k/64] |= 1 << (k % 64)
	}
	return s
}

func (s kindSet) has(k NodeKind) bool { return s[k/64]&(1<<(k%64)) != 0 }

func (s kindSet) or(o ...kindSet) kindSet {
	for _, x := range o {
		s[0] |= x[0]
		s[1] |= x[1]
	}
	return s
}

func (s kindSet) subsetOf(o kindSet) bool { return s[0]&^o[0] == 0 && s[1]&^o[1] == 0 }

func (s kindSet) count() int {
	n := 0
	for k := NodeKind(0); k < kindEnd; k++ {
		if s.has(k) {
			n++
		}
	}
	return n
}

func (s kindSet) only() NodeKind {
	for k := NodeKind(0); k < kindEnd; k++ {
		if s.has(k) {
			return k
		}
	}
	return NoneKind
}

func (s kindSet) list() []NodeKind {
	var out []NodeKind
	for k := NodeKind(0); k < kindEnd; k++ {
		if s.has(k) {
			out = append(out, k)
		}
	}
	return out
}

func ch(name string, s kindSet) slot  { return slot{name: name, role: roleChild, kinds: s} }
func och(name string, s kindSet) slot { return slot{name: name, role: roleChild, kinds: s, opt: true} }
func lst(name string, items kindSet) slot {
	return slot{name: name, role: roleList, kinds: items}
}
func olst(name string, items kindSet) slot {
	return slot{name: name, role: roleList, kinds: items, opt: true}
}
func tk(name string) slot  { return slot{name: name, role: roleTok} }
func otk(name string) slot { return slot{name: name, role: roleTok, opt: true} }
func num(name string) slot { return slot{name: name, role: roleInt} }
func items(s kindSet) slot { return slot{name: "Items", role: roleItems, kinds: s} }
func toks() slot           { return slot{name: "Toks", role: roleToks} }

var anyKind = func() kindSet {
	var s kindSet
	for k := NodeKind(1); k < kindEnd; k++ {
		s[k/64] |= 1 << (k % 64)
	}
	return s
}()

// kindInfo is the per-kind data the tree walkers read; it is derived from
// schema when the package loads.
type kindInfo struct {
	layout layout
	// children holds the Extra offsets of a layRec kind's child slots.
	children []uint8
}

var (
	kindTable   [kindEnd]kindInfo
	kindSchemas [kindEnd]*kindSchema
)

var schema = slices.Concat(exprSchema, patSchema, shellSchema, declSchema, stmtSchema, typeSchema)

func init() {
	for i := range schema {
		ks := &schema[i]
		kindSchemas[ks.kind] = ks
		info := kindInfo{layout: ks.layout}
		if ks.layout == layRec {
			for i, s := range ks.slots {
				if s.role == roleChild || s.role == roleList {
					info.children = append(info.children, uint8(i))
				}
			}
		}
		kindTable[ks.kind] = info
	}
}

var _ [128 - kindEnd]struct{}
