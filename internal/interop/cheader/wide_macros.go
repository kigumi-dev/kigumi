package cheader

import (
	"os"
	"regexp"
	"strings"
)

var (
	reDefine   = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*define[ \t]+(\w+)(?:[ \t]+([^\n]*))?$`)
	reComments = regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`)
)

// translate-c turns L"...", u"..." and U"..." into plain strings, so the macros that define them are found in the source.
func wideStringMacros(closure []SourceFile) map[string]bool {
	wide := map[string]bool{}
	for _, f := range closure {
		b, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}
		src := reComments.ReplaceAllString(strings.ReplaceAll(string(b), "\\\n", " "), " ")
		for _, m := range reDefine.FindAllStringSubmatch(src, -1) {
			if hasWideLiteral(m[2]) {
				wide[m[1]] = true
			}
		}
	}
	return wide
}

func hasWideLiteral(body string) bool {
	for i := 0; i < len(body); i++ {
		switch c := body[i]; c {
		case '"', '\'':
			if c == '"' && wideStringPrefix(body[:i]) {
				return true
			}
			for i++; i < len(body) && body[i] != c; i++ {
				if body[i] == '\\' {
					i++
				}
			}
		}
	}
	return false
}

func wideStringPrefix(before string) bool {
	n := len(before)
	for n > 0 && (before[n-1] == '_' || before[n-1] >= '0' && before[n-1] <= '9' || before[n-1]|0x20 >= 'a' && before[n-1]|0x20 <= 'z') {
		n--
	}
	switch before[n:] {
	case "L", "u", "U":
		return true
	}
	return false
}
