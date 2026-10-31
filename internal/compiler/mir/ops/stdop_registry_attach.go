package ops

import (
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/compiler/sem"
)

type ScalarBoxSource struct {
	Argument int
	TypeArg  int
}

func parseScalarBox(value string) (*ScalarBoxSource, error) {
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return nil, fmt.Errorf("scalar-box wants an argument index and a type-argument index")
	}
	argument, aerr := strconv.Atoi(fields[0])
	typeArg, terr := strconv.Atoi(fields[1])
	if aerr != nil || terr != nil || argument < 0 || typeArg < 0 {
		return nil, fmt.Errorf("scalar-box indices must be non-negative integers")
	}
	return &ScalarBoxSource{Argument: argument, TypeArg: typeArg}, nil
}

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
	case "callback":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("callback wants an argument index, got %q", value)
		}
		op.Callback = n
	case "scalar-box":
		if op.ScalarBox != nil {
			return fmt.Errorf("op %s has scalar-box twice", op.Name)
		}
		var err error
		op.ScalarBox, err = parseScalarBox(value)
		return err
	case "type":
		pkg, name, ok := strings.Cut(value, " ")
		if !ok {
			return fmt.Errorf("type wants PACKAGE NAME")
		}
		op.Types = append(op.Types, NativeType{pkg, name})
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
