package mir

import (
	"fmt"
	"strconv"
	"strings"
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
	}
	return nil
}
