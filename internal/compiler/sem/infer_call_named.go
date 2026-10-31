package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

func (c *checker) hasNamedArg(args []syntax.NodeID) bool {
	for _, a := range args {
		if c.t.Kind(a) == syntax.NamedArg {
			return true
		}
	}
	return false
}

// paramNames covers declParams[:fixed]; the variadic tail stays unnamed.
func (c *checker) paramNames(declParams []EntityID, fixed int) []string {
	n := min(fixed, len(declParams))
	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = c.r.Entities[declParams[i]].Name
	}
	return names
}

type namedLayout struct {
	// order and tail hold indices into the call's args, not the nodes; order[i] is -1 for a slot no argument fills.
	order    []int
	tail     []int
	badOrder []syntax.NodeID
	unknown  []syntax.NodeID
	dup      []syntax.NodeID
}

func (lay namedLayout) ok() bool {
	return len(lay.badOrder) == 0 && len(lay.unknown) == 0 && len(lay.dup) == 0
}

func (lay namedLayout) place() []int {
	return append(append([]int{}, lay.order...), lay.tail...)
}

// Reports no diagnostics itself, so it doubles as a silent viability probe
// for overload candidates.
func (c *checker) layoutNamed(args []syntax.NodeID, names []string) namedLayout {
	lay := namedLayout{order: make([]int, len(names))}
	for i := range lay.order {
		lay.order[i] = -1
	}
	filled := make([]bool, len(names))
	seenNamed, pos := false, 0
	for i, a := range args {
		if c.t.Kind(a) != syntax.NamedArg {
			if seenNamed {
				lay.badOrder = append(lay.badOrder, a)
			} else if pos < len(names) {
				filled[pos] = true
				lay.order[pos] = i
			} else {
				lay.tail = append(lay.tail, i)
			}
			pos++
			continue
		}
		seenNamed = true
		name := c.t.View(a).Tok().Text()
		switch idx := slices.Index(names, name); {
		case idx < 0:
			lay.unknown = append(lay.unknown, a)
		case filled[idx]:
			lay.dup = append(lay.dup, a)
		default:
			filled[idx] = true
			lay.order[idx] = i
		}
	}
	return lay
}

// declParams == nil means a fn-typed value with no parameter names: named args are rejected and stripped.
func (c *checker) resolveNamedArgs(fnName string, args []syntax.NodeID, declParams []EntityID, fixed int) ([]int, bool) {
	if declParams == nil {
		for _, a := range args {
			if c.t.Kind(a) == syntax.NamedArg {
				c.errAt(a, cNamedArgOnFnValue)
			}
		}
		place := make([]int, len(args))
		for i := range place {
			place[i] = i
		}
		return place, true
	}
	lay := c.layoutNamed(args, c.paramNames(declParams, fixed))
	if !lay.ok() {
		c.reportNamedLayout(fnName, lay)
		return nil, false
	}
	return lay.place(), true
}

func (c *checker) unwrapNamed(args []syntax.NodeID) []syntax.NodeID {
	out := make([]syntax.NodeID, len(args))
	for i, a := range args {
		if c.t.Kind(a) == syntax.NamedArg {
			out[i] = c.t.View(a).AsNamedArg().Value().ID()
			if v := c.piped[a]; v != 0 {
				out[i] = v
			}
		} else {
			out[i] = a
		}
	}
	return out
}

func (c *checker) reportNamedLayout(fnName string, lay namedLayout) {
	for _, a := range lay.badOrder {
		c.errAt(a, cPositionalAfterNamed)
	}
	for _, a := range lay.unknown {
		c.errAt(a, cNamedArgUnknown, fnName, c.t.View(a).Tok().Text())
	}
	for _, a := range lay.dup {
		c.errAt(a, cNamedArgDuplicate, c.t.View(a).Tok().Text())
	}
}
