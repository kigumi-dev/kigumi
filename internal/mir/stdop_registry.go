package mir

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/sem"
)

//go:embed stdops.reg
var stdOpRegistrySource string

// StdOpEntry is one op of the registry.
type StdOpEntry struct {
	ID       int
	Name     string
	Provider bool
	Desc     StdOpDesc
	Cap      Capability
	Keys     []string
	Suffixes []string
	Prefixes []string
	Types    []nativeTypeRef
	Direct   string
	Elem     bool
	// Callback is the argument index of a closure the op calls, -1 for none.
	Callback  int
	ScalarBox *ScalarBoxSource
	// Isr is the reviewed interrupt contract (`isr` line); IsrNone when the op has none.
	Isr sem.IsrContract
	// Retired keeps the id taken after the std function gained a Kigumi body; nothing resolves to it.
	Retired bool
}

// BodilessGroup is a `bodiless` entry: std functions lowered without an op that still carry a reviewed contract.
type BodilessGroup struct {
	Name     string
	Keys     []string
	Prefixes []string
	Isr      sem.IsrContract
}

// StdOpRegistry is the parsed registry: ops in id order, plus the builtins and the bodiless groups.
type StdOpRegistry struct {
	Ops      []StdOpEntry
	Builtins []BuiltinSpec
	Bodiless []BodilessGroup
	// inGroup routes key/prefix/isr lines to the last bodiless group instead of the last op.
	inGroup bool
}

// ParseStdOpRegistry parses registry text; an error names the offending line.
func ParseStdOpRegistry(text string) (*StdOpRegistry, error) {
	reg := &StdOpRegistry{}
	for n, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := reg.addLine(line); err != nil {
			return nil, fmt.Errorf("stdops.reg:%d: %w", n+1, err)
		}
	}
	return reg, reg.validate()
}

func (reg *StdOpRegistry) addLine(line string) error {
	word, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)
	switch word {
	case "op":
		reg.inGroup = false
		return reg.addOp(rest)
	case "builtin":
		reg.inGroup = false
		return reg.addBuiltin(rest)
	case "bodiless":
		if rest == "" {
			return fmt.Errorf("bodiless needs a name")
		}
		reg.inGroup = true
		reg.Bodiless = append(reg.Bodiless, BodilessGroup{Name: rest})
		return nil
	}
	if reg.inGroup {
		return reg.attachGroup(&reg.Bodiless[len(reg.Bodiless)-1], word, rest)
	}
	switch word {
	case "cap", "key", "suffix", "prefix", "type", "direct", "elem", "callback", "isr", "scalar-box":
		if len(reg.Ops) == 0 || (rest == "" && word != "elem") {
			return fmt.Errorf("%s needs an op above it and a value", word)
		}
		if last := reg.Ops[len(reg.Ops)-1]; last.Retired {
			return fmt.Errorf("%s: op %s is retired and takes no attributes", word, last.Name)
		}
		return reg.attach(&reg.Ops[len(reg.Ops)-1], word, rest)
	}
	return fmt.Errorf("unknown directive %q", word)
}

func (reg *StdOpRegistry) addOp(rest string) error {
	fields := strings.SplitN(rest, " ", 4)
	if len(fields) < 3 || (len(fields) == 3) != (fields[2] == "retired") {
		return fmt.Errorf("op wants ID NAME KIND (args) -> result, or ID NAME retired")
	}
	id, err := strconv.Atoi(fields[0])
	if err != nil || id != len(reg.Ops) {
		return fmt.Errorf("op %s has id %s, want %d: ids are consecutive from 0", fields[1], fields[0], len(reg.Ops))
	}
	for _, o := range reg.Ops {
		if o.Name == fields[1] {
			return fmt.Errorf("op %s appears twice", o.Name)
		}
	}
	if fields[2] == "retired" {
		reg.Ops = append(reg.Ops, StdOpEntry{ID: id, Callback: -1, Name: fields[1], Retired: true})
		return nil
	}
	if fields[2] != "primitive" && fields[2] != "provider" {
		return fmt.Errorf("op %s: kind %q is neither primitive nor provider", fields[1], fields[2])
	}
	args, result, _, err := parseDescriptor(fields[3])
	if err != nil {
		return fmt.Errorf("op %s: %w", fields[1], err)
	}
	reg.Ops = append(reg.Ops, StdOpEntry{ID: id, Callback: -1, Name: fields[1], Provider: fields[2] == "provider", Desc: StdOpDesc{Args: args, Result: result}})
	return nil
}

func parseDescriptor(text string) (args []ClassSet, result ClassSet, tail string, err error) {
	open, close := strings.Index(text, "("), strings.Index(text, ")")
	if open != 0 || close < 0 {
		return nil, 0, "", fmt.Errorf("want (args) -> result, got %q", text)
	}
	if inner := strings.TrimSpace(text[1:close]); inner != "" {
		for _, a := range strings.Split(inner, ",") {
			set, err := parseClassSet(strings.TrimSpace(a))
			if err != nil {
				return nil, 0, "", err
			}
			args = append(args, set)
		}
	}
	after, ok := strings.CutPrefix(strings.TrimSpace(text[close+1:]), "->")
	if !ok {
		return nil, 0, "", fmt.Errorf("want -> after the arguments in %q", text)
	}
	word, tail, _ := strings.Cut(strings.TrimSpace(after), " ")
	result, err = parseClassSet(word)
	return args, result, strings.TrimSpace(tail), err
}

func mustParseStdOpRegistry() *StdOpRegistry {
	reg, err := ParseStdOpRegistry(stdOpRegistrySource)
	if err != nil {
		panic(err)
	}
	return reg
}

// Registry is the parsed embedded registry, shared read-only.
var Registry = mustParseStdOpRegistry()
