package explain_test

import (
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestExplainFindsSyntaxAndOwnershipCodes(t *testing.T) {
	for _, tc := range []struct{ arg, code, name string }{
		{"E2199", "E2199", "syntax-error"},
		{"syntax-error", "E2199", "syntax-error"},
		{"E2197", "E2197", "use-after-move"},
		{"use-after-move", "E2197", "use-after-move"},
		{"E2198", "E2198", "borrow-conflict"},
		{"E716", "E716", "defer-fail-outside-result"},
	} {
		code, out, errOut := clitest.Execute(t, "explain", tc.arg)
		if code != 0 || !strings.HasPrefix(out, tc.code+" ("+tc.name+")") {
			t.Errorf("explain %s: exit %d\n%s%s", tc.arg, code, out, errOut)
		}
	}
}
