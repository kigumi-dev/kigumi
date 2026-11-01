package sem

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// A duplicate code number would make `--explain` and the docs ambiguous,
// so every defineCode number must be unique.
func TestCodeNumbersUnique(t *testing.T) {
	re := regexp.MustCompile(`defineCode\("(E\d+)", "([a-z0-9-]+)"`)
	seen := map[string]string{}
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			if prev, ok := seen[m[1]]; ok && prev != m[2] {
				t.Errorf("%s defined twice: %s and %s", m[1], prev, m[2])
			}
			seen[m[1]] = m[2]
		}
	}
}

func TestCodesListErrorsFirstByNumber(t *testing.T) {
	list := Codes()
	if len(list) != len(codes) {
		t.Fatalf("Codes lists %d of %d codes", len(list), len(codes))
	}
	for i := 1; i < len(list); i++ {
		a, b := list[i-1], list[i]
		if a.Num[0] > b.Num[0] || a.Num[0] == b.Num[0] && codeNumber(a.Num) >= codeNumber(b.Num) {
			t.Fatalf("%s is listed before %s", a.Num, b.Num)
		}
	}
	if list[0].Num != "E100" {
		t.Fatalf("the listing starts at %s, want E100", list[0].Num)
	}
}
