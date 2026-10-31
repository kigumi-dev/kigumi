package sem

import "kigumi/internal/compiler/syntax"

// abstractAll expands `|` alternatives, including ones nested in
// constructor arguments or record fields, into the list of plain patterns
// an arm stands for.
func (c *checker) abstractAll(p syntax.NodeID) []pat {
	v := c.t.View(p)
	if c.info.Types[p] == TyPoison {
		return []pat{{kind: pErr}}
	}
	switch v.Kind() {
	case syntax.PatOr:
		var out []pat
		for alt := range c.t.List(p).IDs() {
			out = append(out, c.abstractAll(alt)...)
		}
		return out
	case syntax.PatCtor:
		ent := c.info.Uses[p]
		if ent == 0 || c.r.Entities[ent].Kind != EntVariant || !v.AsPatCtor().Args().Valid() {
			return []pat{c.abstract(p)}
		}
		var columns [][]pat
		for s := range v.AsPatCtor().Args().IDs() {
			columns = append(columns, c.abstractAll(s))
		}
		var out []pat
		for _, combo := range product(columns) {
			vi := c.r.variant(ent)
			args := wilds(len(vi.Payload))
			copy(args, combo)
			out = append(out, pat{kind: pCtor, key: variantKey(ent), name: c.r.Entities[ent].Name, args: args})
		}
		return out
	case syntax.PatRecord:
		ent := c.info.Uses[p]
		if ent == 0 {
			return []pat{{kind: pWild}}
		}
		var names []string
		var columns [][]pat
		for fv := range v.AsPatRecord().Fields().Views() {
			f := fv.AsPatField()
			if f.Sub().Valid() {
				names = append(names, f.Name().Text())
				columns = append(columns, c.abstractAll(f.Sub().ID()))
			}
		}
		var out []pat
		for _, combo := range product(columns) {
			subs := map[string]pat{}
			for i, name := range names {
				subs[name] = combo[i]
			}
			out = append(out, c.recordPat(ent, subs))
		}
		return out
	case syntax.PatTuple:
		ent := c.info.Uses[p]
		if ent == 0 {
			return []pat{{kind: pWild}}
		}
		var columns [][]pat
		for s := range c.t.List(p).IDs() {
			columns = append(columns, c.abstractAll(s))
		}
		var out []pat
		for _, combo := range product(columns) {
			out = append(out, pat{kind: pCtor, key: recordKey(ent), name: c.r.Entities[ent].Name, args: combo})
		}
		return out
	}
	return []pat{c.abstract(p)}
}

// product lists every choice of one pattern per column.
func product(columns [][]pat) [][]pat {
	out := [][]pat{nil}
	for _, col := range columns {
		var next [][]pat
		for _, prefix := range out {
			for _, alt := range col {
				row := append(append([]pat{}, prefix...), alt)
				next = append(next, row)
			}
		}
		out = next
	}
	return out
}
