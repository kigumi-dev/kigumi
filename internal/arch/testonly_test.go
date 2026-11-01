package arch_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

var testOnly = []string{"devkit/mirfixture"}

func TestTestOnlyPackagesStayOutOfProductCode(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}{{range .Imports}} {{.}}{{end}}`, "kigumi/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		for _, imp := range fields[1:] {
			if name, ok := strings.CutPrefix(imp, mod); ok && slices.Contains(testOnly, name) {
				t.Errorf("%s が test 専用の %s を import している(_test.go からだけ使え)", fields[0], name)
			}
		}
	}
}
