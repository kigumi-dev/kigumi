package ops

import "fmt"

func (reg *StdOpRegistry) validate() error {
	claimed := map[string]string{}
	vocab := map[Capability]bool{}
	for _, c := range vocabulary {
		vocab[c] = true
	}
	for _, op := range reg.Ops {
		if src := op.ScalarBox; src != nil && (src.Argument >= len(op.Desc.Args) || op.Desc.Args[src.Argument] != classBit(ClassNamed)) {
			return fmt.Errorf("op %s: scalar-box needs a named argument", op.Name)
		}
		if op.Cap != "" && !vocab[op.Cap] {
			return fmt.Errorf("op %s: capability %q is not in the vocabulary", op.Name, op.Cap)
		}
		if _, ok := directPrimByName[op.Direct]; op.Direct != "" && !ok {
			return fmt.Errorf("op %s: %q is not a direct primitive", op.Name, op.Direct)
		}
		if op.Retired && (op.Cap != "" || op.Direct != "" || op.Elem || op.Callback >= 0 || op.ScalarBox != nil || len(op.Keys)+len(op.Suffixes)+len(op.Prefixes)+len(op.Types) > 0) {
			return fmt.Errorf("op %s is retired but still has attributes", op.Name)
		}
		var claims []string
		for _, k := range op.Keys {
			claims = append(claims, "key "+k)
		}
		for _, s := range op.Suffixes {
			claims = append(claims, "suffix "+s)
		}
		for _, p := range op.Prefixes {
			claims = append(claims, "prefix "+p)
		}
		for _, c := range claims {
			if other, dup := claimed[c]; dup {
				return fmt.Errorf("%s is claimed by both %s and %s", c, other, op.Name)
			}
			claimed[c] = op.Name
		}
	}
	return nil
}
