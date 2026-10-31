package llgen

import (
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
)

// leafOf is the field a record-form instruction names; a sum type holds payload slots, not fields.
func leafOf(a *aggInfo, in midir.Inst) aggField {
	if a.isAdt() || in.Index < 0 || in.Index >= len(a.Fields) {
		panic(fmt.Sprintf("llgen: %s names field %d of %s, which has %d", in.Op, in.Index, a.Name, len(a.Fields)))
	}
	return a.Fields[in.Index]
}
