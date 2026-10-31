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
	"compiler/mir/ir":           {"compiler/mir/ops", "compiler/sem", "compiler/syntax", "lang/diag"},
	"compiler/mir/ops":          {"compiler/sem"},
	"arch":                      {},
	"toolchain/native/object":   {},
	"toolchain/native/audit":    {"toolchain/config", "toolchain/native/object"},
	"toolchain/config":          {"lang/target", "toolchain/zig", "toolchain/zig/store"},
	"toolchain/policy":          {},
	"toolchain/version":         {},
	"base/platform":             {},
	"lang/target":               {},
	"toolchain/buildgraph":      {},
	"backend/vm/host/shell":     {"lang/token", "lang/diag", "compiler/syntax"},
	"backend/vm/host/net":       {},
	"devkit/semfixture":         {"lang/token", "compiler/syntax", "compiler/sem", "base/platform"},
	"lang/token":                {},
	"lang/diag":                 {"lang/token"},
	"compiler/syntax":           {"lang/token", "lang/diag"},
	"compiler/printer":          {"lang/token", "compiler/syntax"},
	"compiler/sem/asmtable":     {},
	"compiler/sem":              {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem/asmtable"},
	"compiler/hir":              {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem"},
	"compiler/mir":              {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "compiler/hir", "compiler/mir/ops", "compiler/mir/ir"},
	"backend/llgen":             {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "compiler/mir", "compiler/sem/asmtable", "compiler/mir/ops", "compiler/mir/ir"},
	"base/hashkey":              {},
	"toolchain/zig":             {"toolchain/zig/store"},
	"toolchain/zig/store":       {},
	"toolchain/zig/fetch":       {"toolchain/zig/store"},
	"devkit/zigrelease":         {"toolchain/zig/fetch"},
	"toolchain/zig/source":      {"toolchain/zig/store"},
	"interop/cheader":           {"toolchain/zig"},
	"devkit/fixture":            {},
	"devkit/hosttool":           {},
	"devkit/audit":              {"devkit/fixture", "devkit/hosttool"},
	"devkit/bench":              {"toolchain/version", "devkit/hosttool"},
	"backend/vm":                {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "compiler/mir", "backend/vm/host/shell", "toolchain/buildgraph", "backend/vm/host/net", "base/hashkey", "compiler/mir/ops", "compiler/mir/ir"},
	"toolchain/driver":          {"lang/token", "lang/diag", "compiler/syntax", "compiler/printer", "compiler/sem", "compiler/mir", "backend/llgen", "backend/vm", "toolchain/buildgraph", "toolchain/version", "base/platform", "lang/target", "interop/cheader", "toolchain/contract/schema", "toolchain/zig", "toolchain/zig/store", "toolchain/zig/fetch", "toolchain/policy", "toolchain/config", "toolchain/native/audit", "toolchain/native/object", "compiler/mir/ops"},
	"tooling/lsp":               {"lang/token", "lang/diag", "compiler/syntax", "compiler/printer", "compiler/sem", "toolchain/driver", "base/errors", "toolchain/version", "toolchain/config"},
	"base/errors":               {},
	"cli/exit":                  {"base/errors"},
	"cli/session":               {"cli/exit", "toolchain/driver", "base/errors", "toolchain/zig/store", "toolchain/policy", "toolchain/config"},
	"cli/cmd/parse":             {"cli/exit", "toolchain/driver", "base/errors"},
	"cli/cmd/fmt":               {"cli/exit", "toolchain/driver", "base/errors"},
	"cli/cmd/check":             {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/policy"},
	"cli/cmd/run":               {"cli/session", "cli/exit", "toolchain/driver", "base/errors"},
	"cli/cmd/build":             {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/contract/schema", "toolchain/policy"},
	"cli/cmd/lsp":               {"cli/session", "base/errors", "tooling/lsp"},
	"cli/cmd/doc":               {"cli/session", "cli/exit", "tooling/doc", "toolchain/driver", "base/errors"},
	"cli/cmd/doctor":            {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/version", "toolchain/zig", "toolchain/zig/store"},
	"cli/cmd/explain":           {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "compiler/sem"},
	"cli/cmd/get":               {"cli/session", "cli/exit", "toolchain/driver", "base/errors"},
	"cli/cmd/mod":               {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/version"},
	"cli/cmd/zig":               {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/version", "compiler/sem", "toolchain/zig", "toolchain/zig/store", "toolchain/zig/fetch", "toolchain/zig/source"},
	"toolchain/contract/schema": {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "toolchain/version", "base/platform"},
	"tooling/doc":               {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "toolchain/driver", "base/errors", "toolchain/version", "toolchain/contract/schema"},
	"cli":                       {"toolchain/version", "cli/cmd/build", "cli/cmd/check", "cli/cmd/doc", "cli/cmd/doctor", "cli/cmd/fmt", "cli/cmd/get", "cli/cmd/lsp", "cli/cmd/mod", "cli/cmd/parse", "cli/cmd/run", "cli/exit", "base/errors", "cli/cmd/explain", "cli/session", "lang/diag", "cli/cmd/zig"},
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
