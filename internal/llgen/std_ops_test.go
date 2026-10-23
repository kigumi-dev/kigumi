package llgen

import (
	"fmt"
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

// TestStdOpsMatchC checks the generated `enum std_op` carries the registry's
// ids in order, and that rt_std_id has one case per op (real, or a capability-guarded panic stub) across every runtime file.
func TestStdOpsMatchC(t *testing.T) {
	src := []byte(RuntimeSource())
	block := enumBlockRe.FindSubmatch(src)
	if block == nil {
		t.Fatal("enum std_op { ... } not found in the runtime")
	}
	var enumNames []string
	for i, m := range enumEntryRe.FindAllSubmatch(block[1], -1) {
		enumNames = append(enumNames, string(m[1]))
		if want := fmt.Sprintf("OP_%s = %d,", m[1], i); !strings.Contains(string(block[1]), want) {
			t.Errorf("enum std_op: %s is not at id %d", m[1], i)
		}
	}
	if len(enumNames) != len(mir.StdOpNames) {
		t.Fatalf("enum std_op has %d entries, the registry has %d", len(enumNames), len(mir.StdOpNames))
	}
	for i, name := range mir.StdOpNames {
		if enumNames[i] != name {
			t.Errorf("id %d: the registry has %s, C's enum std_op has %s at the same position", i, name, enumNames[i])
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

var commentLineRe = regexp.MustCompile(`^\s*//`)

// commentRuns returns every run of 3+ consecutive `//` comment lines in src,
// each as the exact lines (including indentation) joined by "\n".
func commentRuns(src string) []string {
	var runs []string
	var cur []string
	flush := func() {
		if len(cur) >= 3 {
			runs = append(runs, strings.Join(cur, "\n"))
		}
		cur = nil
	}
	for _, line := range strings.Split(src, "\n") {
		if commentLineRe.MatchString(line) {
			cur = append(cur, line)
		} else {
			flush()
		}
	}
	flush()
	return runs
}

// TestRuntimeCapFileCommentRuns fails on a capability file's 3+-line comment run.
func TestRuntimeCapFileCommentRuns(t *testing.T) {
	files := append([]RuntimeFile{{"rt_internal.h", runtimeInternalHeader}}, capFiles...)
	for _, f := range files {
		for _, run := range commentRuns(f.Src) {
			t.Errorf("%s has a 3+-line comment run:\n%s", f.Name, run)
		}
	}
}

func TestLlgenGoSourceCommentRuns(t *testing.T) {
	for _, name := range []string{"std_ops_test.go", "runtime_files.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range commentRuns(string(src)) {
			t.Errorf("%s has a 3+-line comment run:\n%s", name, run)
		}
	}
}
