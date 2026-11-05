package llgen

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	cLineComment  = regexp.MustCompile(`//[^\n]*`)
	cBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cSpace        = regexp.MustCompile(`\s+`)
)

// layoutBlocks names the C declarations whose shape the generated code and
// the runtime must agree on: each is a marker that opens a brace block.
var layoutBlocks = []struct{ src, marker string }{
	{"rt.h", "enum { K_INT"},
	{"rt.h", "struct obj {"},
	{"rt.h", "struct rt_type {"},
	{"rt.h", "struct rt_vdesc {"},
	{"rt_core.c", "typedef struct kg_unwind {"},
	{"rt_core.c", "typedef struct kg_uframe {"},
}

// RuntimeLayout is the normalized text of the value layout the runtime and
// the emitted code share: kind enum, object header, type descriptors, the unwind records and the Go-side constants that mirror them.
func RuntimeLayout() (string, error) {
	sources := map[string]string{"rt.h": runtimeHeader, "rt_core.c": runtimeCore}
	var sb strings.Builder
	for _, b := range layoutBlocks {
		text, err := extractBlock(sources[b.src], b.marker)
		if err != nil {
			return "", fmt.Errorf("%s: %w", b.src, err)
		}
		sb.WriteString(normalizeC(text) + "\n")
	}
	fmt.Fprintf(&sb, "RT_OBJ_SLOT %d\nunwindHeaderWords %d\n%s\n", rtObjSlot(), unwindHeaderWords, irTypeLines())
	return sb.String(), nil
}

// irTypeLines is the emitted module's own view of the runtime's structs.
func irTypeLines() string {
	var lines []string
	for _, line := range strings.Split(header, "\n") {
		if strings.HasPrefix(line, "%") || strings.HasPrefix(line, "@") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func rtObjSlot() int {
	var n int
	fmt.Sscanf(cDefine(runtimeHeader, "RT_OBJ_SLOT"), "%d", &n)
	return n
}

func cDefine(src, name string) string {
	m := regexp.MustCompile(`(?m)^#define ` + name + `\s+(\S+)`).FindStringSubmatch(src)
	if m == nil {
		return ""
	}
	return m[1]
}

// extractBlock returns the text from marker through the `;` after its matching close brace.
func extractBlock(src, marker string) (string, error) {
	start := strings.Index(src, marker)
	if start < 0 {
		return "", fmt.Errorf("layout block %q not found", marker)
	}
	depth, seen := 0, false
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
			seen = true
		case '}':
			depth--
		}
		if seen && depth == 0 {
			if end := strings.IndexByte(src[i:], ';'); end >= 0 {
				return src[start : i+end+1], nil
			}
		}
	}
	return "", fmt.Errorf("layout block %q is not closed", marker)
}

func normalizeC(text string) string {
	text = cBlockComment.ReplaceAllString(text, " ")
	text = cLineComment.ReplaceAllString(text, " ")
	return strings.TrimSpace(cSpace.ReplaceAllString(text, " "))
}
