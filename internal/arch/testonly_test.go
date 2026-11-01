package arch_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// toolKit lists the devkit packages a cmd/* tool links and the commands that may import each; any other devkit package is test-only.
var toolKit = map[string][]string{
	"devkit/audit": {"kigumi/cmd/testaudit"},
	"devkit/bench": {"kigumi/cmd/e2ebench"},
	// No cmd/* may import these directly: they reach the tools only through devkit/audit and devkit/bench.
	"devkit/fixture":  nil,
	"devkit/hosttool": nil,
}

func isDevkit(name string) bool { return strings.HasPrefix(name, "devkit/") }

func isTestOnly(name string) bool {
	_, tool := toolKit[name]
	return isDevkit(name) && !tool
}

func devkitImportAllowed(importer, name string) bool {
	from := strings.TrimPrefix(importer, mod)
	if cmds, tool := toolKit[name]; tool {
		return isDevkit(from) || slices.Contains(cmds, importer)
	}
	return isTestOnly(from)
}

func goList(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("go", append([]string{"list"}, args...)...).Output()
	if err != nil {
		t.Fatalf("go list %v: %v", args, err)
	}
	return string(out)
}

func TestDevkitStaysOutOfProductCode(t *testing.T) {
	devkits := 0
	for line := range strings.Lines(goList(t, "-f", `{{.ImportPath}}{{range .Imports}} {{.}}{{end}}`, "kigumi/...")) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if isDevkit(strings.TrimPrefix(fields[0], mod)) {
			devkits++
		}
		for _, imp := range fields[1:] {
			name, ok := strings.CutPrefix(imp, mod)
			if !ok || !isDevkit(name) || devkitImportAllowed(fields[0], name) {
				continue
			}
			if isTestOnly(name) {
				t.Errorf("%s が test 専用の %s を import している(_test.go からだけ使え)", fields[0], name)
			} else {
				t.Errorf("%s が cmd 用の %s を import している(toolKit に載った cmd と devkit だけが使える)", fields[0], name)
			}
		}
	}
	if devkits <= len(toolKit) {
		t.Fatalf("devkit のパッケージが %d 個しか見えない(go list の対象を確かめよ)", devkits)
	}
}

func TestDevkitToolsAreLinkedByCommands(t *testing.T) {
	linked := strings.Fields(goList(t, "-deps", "kigumi/cmd/..."))
	for name := range toolKit {
		if !slices.Contains(linked, mod+name) {
			t.Errorf("%s はどの cmd からも link されていない(test 専用なら toolKit から外せ)", name)
		}
	}
	for _, path := range linked {
		if name, ok := strings.CutPrefix(path, mod); ok && isTestOnly(name) {
			t.Errorf("cmd が test 専用の %s を link している", name)
		}
	}
}
