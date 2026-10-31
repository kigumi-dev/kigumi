package mir

import (
	"fmt"
	"strconv"
	"strings"
)

func (reg *StdOpRegistry) addBuiltin(rest string) error {
	name, descText, ok := strings.Cut(rest, " ")
	if !ok {
		return fmt.Errorf("builtin wants NAME (args) -> result")
	}
	for _, b := range reg.Builtins {
		if b.Name == name {
			return fmt.Errorf("builtin %s appears twice", name)
		}
	}
	args, result, tail, err := parseDescriptor(strings.TrimSpace(descText))
	if err != nil {
		return fmt.Errorf("builtin %s: %w", name, err)
	}
	spec := BuiltinSpec{Name: name, Args: args, Result: result}
	for _, flag := range strings.Fields(tail) {
		if err := spec.setFlag(flag); err != nil {
			return fmt.Errorf("builtin %s: %w", name, err)
		}
	}
	if spec.Min > len(spec.Args) {
		return fmt.Errorf("builtin %s: min=%d exceeds its %d arguments", name, spec.Min, len(spec.Args))
	}
	reg.Builtins = append(reg.Builtins, spec)
	return nil
}

func (s *BuiltinSpec) setFlag(flag string) error {
	switch {
	case flag == "iter":
		s.Iter = true
	case flag == "vm":
		s.VM = true
	case flag == "native=runtime":
		s.Native = NativeRuntime
	case flag == "native=inline":
		s.Native = NativeInline
	case strings.HasPrefix(flag, "min="):
		n, err := strconv.Atoi(strings.TrimPrefix(flag, "min="))
		if err != nil || n < 0 {
			return fmt.Errorf("bad flag %q", flag)
		}
		s.Min = n
	default:
		return fmt.Errorf("unknown flag %q", flag)
	}
	return nil
}
