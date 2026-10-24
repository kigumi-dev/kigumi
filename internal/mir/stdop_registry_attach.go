package mir

import (
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/sem"
)

func (reg *StdOpRegistry) attach(op *StdOpEntry, word, value string) error {
	switch word {
	case "cap":
		op.Cap = Capability(value)
	case "key":
		op.Keys = append(op.Keys, value)
	case "suffix":
		op.Suffixes = append(op.Suffixes, value)
	case "prefix":
		op.Prefixes = append(op.Prefixes, value)
	case "direct":
		op.Direct = value
	case "elem":
		op.Elem = true
	case "retired":
		op.Retired = true
	case "callback":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("callback wants an argument index, got %q", value)
		}
		op.Callback = n
	case "type":
		pkg, name, ok := strings.Cut(value, " ")
		if !ok {
			return fmt.Errorf("type wants PACKAGE NAME")
		}
		op.Types = append(op.Types, nativeTypeRef{pkg, name})
	case "isr":
		c, err := sem.ParseIsrPermission(value)
		if err != nil {
			return fmt.Errorf("op %s: isr: %w", op.Name, err)
		}
		op.Isr = c
	}
	return nil
}

func (reg *StdOpRegistry) attachGroup(g *BodilessGroup, word, value string) error {
	switch word {
	case "key":
		g.Keys = append(g.Keys, value)
	case "prefix":
		g.Prefixes = append(g.Prefixes, value)
	case "isr":
		c, err := sem.ParseIsrPermission(value)
		if err != nil {
			return fmt.Errorf("bodiless %s: isr: %w", g.Name, err)
		}
		g.Isr = c
	default:
		return fmt.Errorf("bodiless %s takes key, prefix and isr lines, not %q", g.Name, word)
	}
	return nil
}
