package mir

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *callVerifier) witnessCallMismatch(f *midir.Func, in midir.Inst, recv bool) string {
	tt := p.R.Types
	t := p.fnType(p.derefAll(f.Locals[in.Args[0]].Type))
	args := in.Args[1:]
	if recv {
		args = args[1:]
	}
	if node := tt.Node(t); node.Kind == sem.KFn {
		if len(node.Args) != len(args) {
			return "passes " + itoa(len(args)) + " values to a witness of type " + p.R.TypeString(t)
		}
		for k, want := range node.Args {
			if !p.compatibleGeneric(want, f.Locals[args[k]].Type) {
				return "passes " + p.R.TypeString(f.Locals[args[k]].Type) + " as argument " + itoa(k) + " of the witness of type " + p.R.TypeString(t)
			}
		}
		if !p.compatibleGeneric(node.Elem, f.Locals[in.Dst].Type) {
			return "takes " + p.R.TypeString(f.Locals[in.Dst].Type) + " from a witness of type " + p.R.TypeString(t)
		}
	}
	if !recv {
		return ""
	}
	param, ok := p.witnessParam(f, in.Args[0], 0)
	if !ok {
		return ""
	}
	got := p.derefAll(f.Locals[in.Args[1]].Type)
	switch {
	case tt.Kind(got) == sem.KParam:
		if p.R.Entity(tt.Node(got).Ent).Name != param {
			return "hands the witness for " + param + " a " + p.R.TypeString(got)
		}
	case !p.looseType(got) && p.mentionsParam(f, param):
		return "hands the witness for " + param + " a " + p.R.TypeString(got)
	}
	return ""
}

func (p *callVerifier) witnessParam(f *midir.Func, l midir.LocalID, depth int) (string, bool) {
	if name := f.Locals[l].Name; strings.HasPrefix(name, "$w:") && isParam(f, l) {
		rest := strings.TrimPrefix(name, "$w:")
		if i := strings.IndexByte(rest, ':'); i > 0 {
			return rest[:i], true
		}
		return "", false
	}
	if depth > 4 {
		return "", false
	}
	src, found := midir.LocalID(0), false
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() || in.Dst != l {
				continue
			}
			switch in.Op {
			case midir.OpAlias, midir.OpCopy, midir.OpShare, midir.OpMove:
				if found && src != in.Args[0] {
					return "", false
				}
				src, found = in.Args[0], true
			default:
				return "", false
			}
		}
	}
	if !found {
		return "", false
	}
	return p.witnessParam(f, src, depth+1)
}

func (p *callVerifier) mentionsParam(f *midir.Func, name string) bool {
	tt := p.R.Types
	for _, l := range f.Locals {
		t := p.derefAll(l.Type)
		if tt.Kind(t) == sem.KParam && p.R.Entity(tt.Node(t).Ent).Name == name {
			return true
		}
	}
	return false
}
