package sem

import (
	"fmt"
	"testing"
)

func TestFillCodeRepeatsAnEarlierHoleOfTheSameName(t *testing.T) {
	str := func(a any) string { return fmt.Sprint(a) }
	cases := []struct {
		code      Code
		args      []any
		msg, help string
	}{
		{cDefinedWrapOutside, []any{"Id", "Id"}, "`Id(...)` is a representation operation usable only inside the package that declares `Id`", "use the functions that package exports for `Id` instead"},
		{cDefinedWrapOutside, []any{"Id"}, "`Id(...)` is a representation operation usable only inside the package that declares `Id`", "use the functions that package exports for `Id` instead"},
		{cCompoundResultType, []any{"+", "Money", "Money", "Int"}, "`+` on (`Money`, `Money`) yields `Int`, not `Money`", ""},
		{cDefinedValueClash, nil, "`value` is `{name}`'s representation projection and cannot also be a method name", ""},
	}
	for _, c := range cases {
		msg, help := fillCode(c.code, str, c.args)
		if msg != c.msg || help != c.help {
			t.Errorf("%s with %v: got %q / %q, want %q / %q", c.code.Num, c.args, msg, help, c.msg, c.help)
		}
	}
}
