package mir

import (
	"strconv"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/syntax"
)

// Site is a minimal source location (file, 1-based line, enclosing function), shared by VM and native so a panic's frame reads the same either way.
type Site struct {
	File string
	Line int
	Func string
}

// SiteAt resolves node's source location within fn.
func (p *Program) SiteAt(fn *midir.Func, node syntax.NodeID) Site {
	if fn == nil || node == 0 {
		return Site{}
	}
	tree := p.R.Tree(fn.File)
	if tree == nil || tree.File == nil {
		return Site{Func: fn.Name}
	}
	sp := tree.Span(node)
	return Site{File: tree.File.Name, Line: tree.File.Line(sp.Start), Func: fn.Name}
}

// EntitySite is SiteAt for ent's own declaration, for callers (test
// listing) that have an entity rather than a mid-body instruction.
func (p *Program) EntitySite(fn *midir.Func) Site {
	ent := p.R.Entity(fn.Ent)
	return p.SiteAt(fn, ent.Node)
}

// Text is `file:line`, or empty for an unknown site.
func (s Site) Text() string {
	if s.File == "" {
		return ""
	}
	return s.File + ":" + strconv.Itoa(s.Line)
}
