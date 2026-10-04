// Package arch holds the repository-wide structure tests: the import layer
// table and the per-file line cap.
package arch_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const mod = "kigumi/internal/"

var allowed = map[string][]string{
	"arch":        {},
	"version":     {},
	"platform":    {},
	"target":      {},
	"buildgraph":  {},
	"hostshell":   {"token", "diag", "syntax"},
	"netsock":     {},
	"testkit":     {"token", "syntax", "sem", "platform"},
	"token":       {},
	"diag":        {"token"},
	"syntax":      {"token", "diag"},
	"printer":     {"token", "syntax"},
	"sem":         {"token", "diag", "syntax"},
	"hir":         {"token", "diag", "syntax", "sem"},
	"mir":         {"token", "diag", "syntax", "sem", "hir"},
	"llgen":       {"token", "diag", "syntax", "sem", "mir"},
	"hashkey":     {},
	"cheader":     {},
	"testtool":    {"version"},
	"vm":          {"token", "diag", "syntax", "sem", "mir", "hostshell", "buildgraph", "netsock", "hashkey"},
	"driver":      {"token", "diag", "syntax", "printer", "sem", "mir", "llgen", "vm", "buildgraph", "version", "platform", "target", "cheader"},
	"lsp":         {"token", "diag", "syntax", "printer", "sem", "driver", "errors", "version"},
	"errors":      {},
	"cliutil":     {"errors"},
	"cli/shared":  {"cliutil", "driver", "errors"},
	"cli/parse":   {"cliutil", "driver", "errors"},
	"cli/fmtcmd":  {"cliutil", "driver", "errors"},
	"cli/check":   {"cli/shared", "cliutil", "driver", "errors"},
	"cli/run":     {"cli/shared", "cliutil", "driver", "errors"},
	"cli/build":   {"cli/shared", "cliutil", "driver", "errors"},
	"cli/lsp":     {"cli/shared", "errors", "lsp"},
	"cli/doc":     {"cli/shared", "cliutil", "doc", "driver", "errors"},
	"cli/doctor":  {"cli/shared", "cliutil", "driver", "errors", "version"},
	"cli/explain": {"sem", "cliutil", "errors"},
	"cli/get":     {"cli/shared", "cliutil", "driver", "errors"},
	"cli/mod":     {"cliutil", "driver", "errors", "version"},
	"doc":         {"token", "diag", "syntax", "sem", "driver", "errors", "version"},
	"cli":         {"version", "cli/build", "cli/check", "cli/doc", "cli/doctor", "cli/fmtcmd", "cli/get", "cli/lsp", "cli/mod", "cli/parse", "cli/run", "cliutil", "errors", "cli/explain", "cli/shared", "diag"},
}

func TestDependencyDAG(t *testing.T) {
	out, err := exec.Command("go", "list", "kigumi/internal/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for pkgPath := range strings.FieldsSeq(string(out)) {
		pkg := strings.TrimPrefix(pkgPath, mod)
		ok, known := allowed[pkg]
		if !known {
			t.Errorf("%s は層表にない(internal/arch/deps_test.go の層表を更新せよ)", pkg)
			continue
		}
		imports, err := exec.Command("go", "list", "-f",
			`{{range .Imports}}{{.}}{{"\n"}}{{end}}`, pkgPath).Output()
		if err != nil {
			t.Fatalf("go list %s: %v", pkg, err)
		}
		for imp := range strings.FieldsSeq(string(imports)) {
			name, internal := strings.CutPrefix(imp, mod)
			if !internal {
				continue
			}
			if !slices.Contains(ok, name) {
				t.Errorf("%s は %s をimportできない(層表を見よ)", pkg, name)
			}
		}
	}
}
