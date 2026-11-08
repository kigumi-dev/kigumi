package driver_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const graphemeConformanceLib = `import array from std/array
import text from std/text

fn lens(xs: &Array[String]) -> String {
    let mut parts = Array.tryWithCapacity[String](0).unwrap()
    for x in xs {
        parts.push("${x.len()}")
    }
    array.join(&parts, ",")
}

fn walk(s: String) -> String {
    let mut b = text.breaker()
    let mut parts = Array.tryWithCapacity[String](0).unwrap()
    let mut size = 0
    let mut first = true
    for c in s.chars() {
        let step = b.step(c)
        if step.boundary && !first {
            parts.push("${size}")
            size = 0
        }
        size = size + ("${c}".len().toInt() || 0)
        first = false
        b = step.next
    }
    if !first {
        parts.push("${size}")
    }
    array.join(&parts, ",")
}

fn check(i: Int, s: String, want: String) -> Int {
    let a = lens(&text.graphemes(s))
    let b = walk(s)
    if a == want && b == want {
        return 0
    }
    print "case ${i}: want ${want} graphemes ${a} breaker ${b}"
    1
}
`

// TestGraphemeBreakConformance runs every case of the vendored GraphemeBreakTest through
// text.graphemes and text.breaker on both engines.
func TestGraphemeBreakConformance(t *testing.T) {
	src, cases := graphemeConformanceProgram(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"gbt\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("cases=%d fails=0\n", cases)
	if out, errOut, code := runFixtureVM(t, root); out != want || code != 0 {
		t.Errorf("vm: exit %d\n%s%s", code, out, errOut)
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler for the native build")
	}
	if out, errOut, code := runFixtureNative(t, root); out != want || code != 0 {
		t.Errorf("native: exit %d\n%s%s", code, out, errOut)
	}
}

func graphemeConformanceProgram(t *testing.T) (string, int) {
	data, err := os.ReadFile("../../../testdata/unicode/GraphemeBreakTest-18.0.0.txt")
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		var s strings.Builder
		var lens []string
		size := 0
		for _, f := range fields {
			switch f {
			case "÷":
				if size > 0 {
					lens = append(lens, fmt.Sprint(size))
					size = 0
				}
			case "×":
			default:
				var r rune
				if _, err := fmt.Sscanf(f, "%X", &r); err != nil {
					t.Fatalf("bad field %q in %q", f, line)
				}
				fmt.Fprintf(&s, "\\u{%x}", r)
				size += len(string(r))
			}
		}
		calls = append(calls, fmt.Sprintf("check(%d, \"%s\", \"%s\")", len(calls), s.String(), strings.Join(lens, ",")))
	}
	var sb strings.Builder
	sb.WriteString(graphemeConformanceLib)
	const chunk = 100
	var chunks []string
	for i := 0; i < len(calls); i += chunk {
		end := min(i+chunk, len(calls))
		fmt.Fprintf(&sb, "\nfn chunk%d() -> Int {\n    let mut f = 0", i/chunk)
		for _, c := range calls[i:end] {
			sb.WriteString("\n    f = f + " + c)
		}
		sb.WriteString("\n    f\n}\n")
		chunks = append(chunks, fmt.Sprintf("chunk%d()", i/chunk))
	}
	fmt.Fprintf(&sb, "\nfn main() {\n    let fails = %s\n    print \"cases=%d fails=${fails}\"\n}\n", strings.Join(chunks, " + "), len(calls))
	return sb.String(), len(calls)
}
