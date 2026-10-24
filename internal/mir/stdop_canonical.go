package mir

import (
	"fmt"
	"strings"
)

// Canonical renders the registry's content (ids, names, kinds, descriptors,
// capabilities, keys, builtins) with no formatting or comments, for hashing.
func (reg *StdOpRegistry) Canonical() string {
	var sb strings.Builder
	for _, op := range reg.Ops {
		if op.Retired {
			fmt.Fprintf(&sb, "op %d %s retired\n", op.ID, op.Name)
			continue
		}
		kind := "primitive"
		if op.Provider {
			kind = "provider"
		}
		fmt.Fprintf(&sb, "op %d %s %s %s\n", op.ID, op.Name, kind, op.Desc)
		fmt.Fprintf(&sb, "cap %s\nkeys %s\nsuffixes %s\nprefixes %s\ntypes %v\ndirect %s elem %t callback %d\n", op.Cap, strings.Join(op.Keys, ","), strings.Join(op.Suffixes, ","), strings.Join(op.Prefixes, ","), op.Types, op.Direct, op.Elem, op.Callback)
	}
	for _, b := range reg.Builtins {
		fmt.Fprintf(&sb, "builtin %s %s min=%d iter=%t vm=%t native=%d\n", b.Name, b.Desc(), b.Min, b.Iter, b.VM, b.Native)
	}
	return sb.String()
}

func (d StdOpDesc) String() string {
	args := make([]string, len(d.Args))
	for i, a := range d.Args {
		args[i] = a.String()
	}
	return "(" + strings.Join(args, ", ") + ") -> " + d.Result.String()
}

// Desc is the builtin's argument and result classes as a StdOpDesc.
func (b BuiltinSpec) Desc() StdOpDesc { return StdOpDesc{Args: b.Args, Result: b.Result} }
