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

// TestStdOpsMatchC checks rt_core.c's `enum std_op` matches mir.StdOpNames,
// and that rt_std_id has one case per op (real, or a capability-guarded panic stub) across every runtime file.
func TestStdOpsMatchC(t *testing.T) {
	src := []byte(RuntimeSource())
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

// longCommentAllowlist holds the 3+-line comment runs moved verbatim out of
// rt_core.c by the split; anything else counts as new and must stay under 3 lines.
var longCommentAllowlist = map[string][]string{
	"rt_cap_ffi_cstring.c.inc": {
		"    // Every String/Bytes buffer already carries a guaranteed trailing\n" +
			"    // NUL at data[len] (see strdup_val); strchr for the first NUL byte\n" +
			"    // reaches that one unless an earlier, embedded NUL is hit first.",
	},
	"rt_cap_ffi_pin.c.inc": {
		"    // mir now consumes self for a type's own drop (isOwnDrop in\n" +
			"    // internal/mir/plan_args.go), so scope exit no longer re-enters\n" +
			"    // here; the zero-guard just stays as a defensive no-op on repeat.",
		"    // release() (unlike drop) still lends self, so the caller's\n" +
			"    // scope-exit drop runs after this; retain here keeps the\n" +
			"    // extracted pointer alive through that drop.",
	},
	"rt_cap_ffi_raw.c.inc": {
		"// Decodes one codepoint at s[i], the same rules utf8_valid checks; returns\n" +
			"// the sequence length on success or 0 on any violation, so a caller can\n" +
			"// tell exactly where a bad sequence starts.",
		"// Lossy UTF-8 conversion for ffi.stringLossy: each byte that\n" +
			"// doesn't start a valid sequence becomes its own U+FFFD, matching Go's\n" +
			"// utf8.DecodeRuneInString fallback, so native agrees byte for byte with\n" +
			"// the VM/interp Go paths.",
	},
	"rt_cap_host_net.c.inc": {
		"// Bodiless like Conn.drop/Listener.drop themselves (see net_hosted.kg):\n" +
			"// an accelerator on a *bodied* drop only replaces normal calls to it,\n" +
			"// not the implicit drop on a resource's last reference going, which\n" +
			"// always runs the real (here nonexistent) lowered body instead.",
	},
}

// TestRuntimeCapFileCommentRuns fails on a capability file's 3+-line comment
// run that isn't in longCommentAllowlist, i.e. not a verbatim move from rt_core.c.
func TestRuntimeCapFileCommentRuns(t *testing.T) {
	files := append([]RuntimeFile{{"rt_internal.h", runtimeInternalHeader}}, capFiles...)
	for _, f := range files {
		allowed := map[string]bool{}
		for _, run := range longCommentAllowlist[f.Name] {
			allowed[run] = true
		}
		for _, run := range commentRuns(f.Src) {
			if !allowed[run] {
				t.Errorf("%s has an unlisted 3+-line comment run:\n%s", f.Name, run)
			}
		}
	}
}

// goCommentRunAllowlist holds this package's pre-existing 3+-line doc
// comments; new ones must stay under 3 lines.
var goCommentRunAllowlist = map[string]bool{
	"// RuntimeFiles includes rt_signal.c only for hosted POSIX OSes: sigaction\n" +
		"// needs a POSIX <signal.h>, which freestanding/\"--sys none\" and\n" +
		"// Windows/WASI lack.": true,
}

func TestLlgenGoSourceCommentRuns(t *testing.T) {
	for _, name := range []string{"std_ops_test.go", "runtime_files.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range commentRuns(string(src)) {
			if !goCommentRunAllowlist[run] {
				t.Errorf("%s has an unlisted 3+-line comment run:\n%s", name, run)
			}
		}
	}
}
