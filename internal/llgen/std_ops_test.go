package llgen

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/mir"
)

var (
	enumBlockRe = regexp.MustCompile(`(?s)enum std_op \{(.*?)\};`)
	enumEntryRe = regexp.MustCompile(`OP_([A-Z0-9_]+)`)
	caseLabelRe = regexp.MustCompile(`case OP_([A-Z0-9_]+):`)
	ppIfRe      = regexp.MustCompile(`^#\s*(if|ifdef|ifndef)\b`)
	ppElseRe    = regexp.MustCompile(`^#\s*(elif|else)\b`)
	ppEndifRe   = regexp.MustCompile(`^#\s*endif\b`)
)

// ppFrame is one open #if/#ifdef/#ifndef: group identifies the directive
// that opened it, branch counts #elif/#else siblings seen since.
type ppFrame struct{ group, branch int }

// A `case OP_X:` repeated in a capability guard's #else panic stub is not a duplicate.
func caseSites(src []byte) map[string][][]ppFrame {
	sites := map[string][][]ppFrame{}
	var stack []ppFrame
	nextGroup := 0
	for _, line := range strings.Split(string(src), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case ppIfRe.MatchString(t):
			stack = append(stack, ppFrame{group: nextGroup})
			nextGroup++
		case ppElseRe.MatchString(t):
			if n := len(stack); n > 0 {
				stack[n-1].branch++
			}
		case ppEndifRe.MatchString(t):
			if n := len(stack); n > 0 {
				stack = stack[:n-1]
			}
		}
		for _, m := range caseLabelRe.FindAllStringSubmatch(line, -1) {
			sites[m[1]] = append(sites[m[1]], append([]ppFrame(nil), stack...))
		}
	}
	return sites
}

func exclusive(a, b []ppFrame) bool {
	at := map[int]int{}
	for _, f := range a {
		at[f.group] = f.branch
	}
	for _, f := range b {
		if branch, ok := at[f.group]; ok && branch != f.branch {
			return true
		}
	}
	return false
}

// TestStdOpsMatchC checks rt_core.c's `enum std_op` matches mir.StdOpNames
// in order, and that rt_std_id has exactly one case per declared op, or
// a capability-guarded pair of mutually exclusive ones (real impl / panic stub).
func TestStdOpsMatchC(t *testing.T) {
	src, err := os.ReadFile("runtime/rt_core.c")
	if err != nil {
		t.Fatal(err)
	}
	block := enumBlockRe.FindSubmatch(src)
	if block == nil {
		t.Fatal("enum std_op { ... } not found in rt_core.c")
	}
	var enumNames []string
	for _, m := range enumEntryRe.FindAllSubmatch(block[1], -1) {
		enumNames = append(enumNames, string(m[1]))
	}
	if len(enumNames) != len(mir.StdOpNames) {
		t.Fatalf("enum std_op has %d entries, mir.StdOpNames has %d", len(enumNames), len(mir.StdOpNames))
	}
	for i, name := range mir.StdOpNames {
		if enumNames[i] != name {
			t.Errorf("id %d: Go has %s, C's enum std_op has %s at the same position", i, name, enumNames[i])
		}
	}
	cases := caseSites(src)
	declared := map[string]bool{}
	for _, name := range enumNames {
		declared[name] = true
		sites := cases[name]
		if len(sites) == 0 {
			t.Errorf("OP_%s is declared but rt_std_id has no case for it", name)
			continue
		}
		for i := range sites {
			for j := i + 1; j < len(sites); j++ {
				if !exclusive(sites[i], sites[j]) {
					t.Errorf("OP_%s has %d cases in rt_std_id, not all mutually exclusive", name, len(sites))
				}
			}
		}
	}
	for name := range cases {
		if !declared[name] {
			t.Errorf("rt_std_id has a case for OP_%s, which enum std_op does not declare", name)
		}
	}
}
