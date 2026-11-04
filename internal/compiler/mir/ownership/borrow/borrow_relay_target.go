package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

type relayKind uint8

const (
	relayNone relayKind = iota
	relayParam
	relayUnknown
)

type relayTarget struct {
	kind  relayKind
	param int
}

func (c *borrowCheck) relaySite(g *midir.Func, defs map[midir.LocalID]*midir.Inst, s paramStore, args paramLocals, kind siteKind, depth int) []paramStore {
	if s.callee.kind != relayParam {
		return []paramStore{s}
	}
	a, ok := args(s.callee.param)
	if !ok {
		s.callee = relayTarget{kind: relayUnknown}
		return []paramStore{s}
	}
	var out []paramStore
	for _, target := range c.relayTargets(g, defs, a, kind, depth) {
		next := s
		next.callee = target
		out = append(out, next)
	}
	return out
}

func (c *borrowCheck) frameTarget(g *midir.Func, defs map[midir.LocalID]*midir.Inst, target relayTarget, args paramLocals, depth int) bool {
	switch target.kind {
	case relayNone:
		return false
	case relayParam:
		a, ok := args(target.param)
		return !ok || c.frameCallee(g, defs, a, depth)
	default:
		return true
	}
}
