package driver_test

import (
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// step is one applied edit: where it landed (a function, a block, an instruction and the opcode found there) and the choices it made.
// A recipe replays on any MIR that still has that site, so a change in the planner cannot pin a different mutation under the same name.
type step struct {
	edit   string
	fnIdx  int
	fnName string
	fnNth  int
	blk    int
	inst   int
	anchor string
	picks  []pick
}

type recipe []step

func allFuncs(p *mir.Program) []*midir.Func {
	fs := append([]*midir.Func{}, p.Funcs...)
	for _, cf := range p.Comptime {
		fs = append(fs, cf.Func)
	}
	return fs
}

func editNamed(name string) (edit, bool) {
	for _, e := range allEdits {
		if e.name == name {
			return e, true
		}
	}
	return edit{}, false
}

func anchorOf(s *site) string {
	if s.hasInst() {
		return s.inst().Op.String()
	}
	return "term"
}

// mutateRecorded applies the edits a generator picks and returns what it did.
func (m *mutator) mutateRecorded(p *mir.Program, rnd *rand.Rand) (string, recipe) {
	c := &chooser{rnd: rnd}
	n := 1
	if c.draw(4) == 0 {
		n++
		if c.draw(4) == 0 {
			n++
		}
	}
	var did []string
	var rec recipe
	for attempt := 0; len(did) < n && attempt < 80; attempt++ {
		c.rec = c.rec[:0]
		s := m.pickSite(p, c)
		e := allEdits[c.draw(len(allEdits))]
		if e.inst && !s.hasInst() {
			continue
		}
		anchor := anchorOf(s)
		if d := e.apply(s); d != "" {
			did = append(did, fmt.Sprintf("%s: %s", e.name, d))
			rec = append(rec, step{edit: e.name, fnIdx: s.fnIdx, blk: s.bi, inst: s.ii, anchor: anchor, picks: slices.Clone(c.rec)})
		}
	}
	if len(did) == 0 {
		return "no edit applied", nil
	}
	return strings.Join(did, "; "), rec
}

// replay applies a recipe to p, which must still hold every site the recipe names.
func (m *mutator) replay(p *mir.Program, rec recipe) (what string, err error) {
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(staleRecipe); ok {
				err = s
				return
			}
			panic(r)
		}
	}()
	var did []string
	for i, st := range rec {
		e, ok := editNamed(st.edit)
		if !ok {
			return "", staleRecipe("no edit named " + st.edit)
		}
		fs := allFuncs(p)
		idx, seen := -1, 0
		for k, f := range fs {
			if f.Name == st.fnName {
				if seen == st.fnNth {
					idx = k
					break
				}
				seen++
			}
		}
		if idx < 0 {
			return "", staleRecipe(fmt.Sprintf("step %d: no function %q#%d", i, st.fnName, st.fnNth))
		}
		f := fs[idx]
		if st.blk >= len(f.Blocks) || st.inst >= len(f.Blocks[st.blk].Insts) {
			return "", staleRecipe(fmt.Sprintf("step %d: %s has no b%d/%d", i, f.Name, st.blk, st.inst))
		}
		c := &chooser{script: st.picks}
		s := &site{m: m, p: p, r: c, f: f, fnIdx: idx, bi: st.blk, ii: st.inst}
		if got := anchorOf(s); got != st.anchor {
			return "", staleRecipe(fmt.Sprintf("step %d: %s b%d/%d is %s now, the recipe edits a %s", i, f.Name, st.blk, st.inst, got, st.anchor))
		}
		d := e.apply(s)
		if d == "" {
			return "", staleRecipe(fmt.Sprintf("step %d: %s no longer applies at %s b%d/%d", i, st.edit, f.Name, st.blk, st.inst))
		}
		if c.next != len(st.picks) {
			return "", staleRecipe(fmt.Sprintf("step %d: %s used %d of %d recorded choices", i, st.edit, c.next, len(st.picks)))
		}
		did = append(did, fmt.Sprintf("%s: %s", st.edit, d))
	}
	return strings.Join(did, "; "), nil
}

// format writes a recipe as text, one step a line; p is the program the recipe was recorded on, or any program the same names resolve in.
func (rec recipe) format(p *mir.Program) string {
	fs := allFuncs(p)
	var lines []string
	for _, st := range rec {
		f := fs[st.fnIdx]
		nth := 0
		for _, g := range fs[:st.fnIdx] {
			if g.Name == f.Name {
				nth++
			}
		}
		site := fmt.Sprintf("b%d/%d", st.blk, st.inst)
		if st.inst < 0 {
			site = fmt.Sprintf("b%d/term", st.blk)
		}
		line := fmt.Sprintf("%s %s %s %s", st.edit, quoteNth(f.Name, nth), site, st.anchor)
		if len(st.picks) > 0 {
			var parts []string
			for _, pk := range st.picks {
				parts = append(parts, pk.format(p, f))
			}
			line += " | " + strings.Join(parts, " ")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func quoteNth(text string, nth int) string {
	if nth == 0 {
		return strconv.Quote(text)
	}
	return strconv.Quote(text) + "#" + strconv.Itoa(nth)
}

func (pk pick) format(p *mir.Program, f *midir.Func) string {
	switch pk.kind {
	case 'i':
		return "i" + strconv.Itoa(pk.n)
	case 'l':
		if pk.recorded {
			pk = nameOfLocal(f, pk.n)
		}
		if pk.oob {
			return "l+" + strconv.Itoa(pk.nth)
		}
		return "l" + quoteNth(pk.text, pk.nth)
	case 't':
		if pk.recorded {
			pk.text, pk.nth = nameOfType(p, sem.TypeID(pk.n))
		}
		return "t" + quoteNth(pk.text, pk.nth)
	case 'e':
		if pk.recorded {
			pk.text, pk.nth = nameOfEntity(p, sem.EntityID(pk.n))
		}
		return "e" + entityKindNames[pk.ekind] + ":" + quoteNth(pk.text, pk.nth)
	case 's':
		return "s" + strconv.Quote(pk.text)
	case 'p':
		return "p" + strconv.Quote(opdef.StdOpNames[pk.n-1])
	default:
		return "v" + strconv.Quote(opdef.StdOpNames[pk.n-1])
	}
}
