package mir

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

func (c *borrowCheck) relaySite(g *midir.Func, defs map[midir.LocalID]*midir.Inst, s paramStore, args []midir.LocalID, kind siteKind, depth int) []paramStore {
	if s.callee.kind != relayParam {
		return []paramStore{s}
	}
	if s.callee.param >= len(args) {
		s.callee = relayTarget{kind: relayUnknown}
		return []paramStore{s}
	}
	var out []paramStore
	for _, target := range c.relayTargets(g, defs, args[s.callee.param], kind, depth) {
		next := s
		next.callee = target
		out = append(out, next)
	}
	return out
}

func (c *borrowCheck) frameTarget(g *midir.Func, defs map[midir.LocalID]*midir.Inst, target relayTarget, args []midir.LocalID, depth int) bool {
	switch target.kind {
	case relayNone:
		return false
	case relayParam:
		return target.param >= len(args) || c.frameCallee(g, defs, args[target.param], depth)
	default:
		return true
	}
}
