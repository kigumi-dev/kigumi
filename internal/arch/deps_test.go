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
	"compiler/mir/flow":             {"compiler/mir/ir"},
	"compiler/mir/ownership":        {"compiler/mir/ir", "compiler/mir/flow", "compiler/sem", "compiler/syntax", "lang/diag"},
	"compiler/mir/ownership/borrow": {"compiler/mir/ir", "compiler/mir/ownership", "compiler/mir/flow", "compiler/sem", "compiler/syntax", "lang/diag"},
	"compiler/mir/resource":         {"compiler/mir/ir", "compiler/mir/layout", "compiler/mir/ops", "compiler/sem", "lang/diag"},
	"compiler/mir/layout":           {"compiler/mir/ir", "compiler/sem"},
	"compiler/mir/ir":               {"compiler/mir/ops", "compiler/sem", "compiler/syntax", "lang/diag"},
	"compiler/mir/ops":              {"compiler/sem", "lang/isr"},
	"compiler/mir/verify":           {"lang/diag", "compiler/syntax", "compiler/sem", "compiler/mir", "compiler/mir/flow", "compiler/mir/ir", "compiler/mir/ops", "compiler/mir/ownership"},
	"compiler/mir/plan":             {"lang/diag", "compiler/syntax", "compiler/sem", "compiler/mir", "compiler/mir/flow", "compiler/mir/ir", "compiler/mir/layout", "compiler/mir/mobility", "compiler/mir/ops", "compiler/mir/ownership", "compiler/mir/unwind", "compiler/mir/verify"},
	"arch":                          {},
	"toolchain/native/object":       {},
	"toolchain/native/audit":        {"toolchain/config", "toolchain/native/object"},
	"toolchain/native/link":         {"toolchain/config", "toolchain/zig"},
	"toolchain/config":              {"lang/target", "toolchain/zig", "toolchain/zig/store"},
	"toolchain/module":              {"lang/token", "lang/diag", "lang/isr", "compiler/syntax", "toolchain/config", "toolchain/version", "base/fsx"},
	"toolchain/module/vcs":          {"toolchain/module", "base/errors"},
	"toolchain/artifact":            {"base/fsx", "toolchain/contract/schema"},
	"toolchain/policy":              {},
	"toolchain/version":             {},
	"base/platform":                 {},
	"lang/target":                   {},
	"toolchain/buildgraph":          {},
	"backend/vm/host/shell":         {"lang/token", "lang/diag", "compiler/syntax"},
	"backend/vm/host/net":           {},
	"devkit/semfixture":             {"lang/token", "compiler/syntax", "compiler/sem", "base/platform"},
	"devkit/mirfixture":             {"lang/token", "compiler/syntax", "compiler/sem", "compiler/mir", "compiler/mir/build", "compiler/mir/ir", "compiler/mir/ops", "compiler/mir/plan", "compiler/mir/verify", "devkit/semfixture", "toolchain/contract/schema"},
	"lang/token":                    {},
	"lang/diag":                     {"lang/token"},
	"lang/isr":                      {},
	"compiler/syntax":               {"lang/token", "lang/diag"},
	"compiler/printer":              {"lang/token", "compiler/syntax"},
	"compiler/sem/asmtable":         {},
	"compiler/sem":                  {"lang/token", "lang/diag", "lang/isr", "compiler/syntax", "compiler/sem/asmtable"},
	"compiler/hir":                  {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem"},
	"compiler/mir":                  {"lang/diag", "compiler/syntax", "compiler/sem", "compiler/mir/ops", "compiler/mir/ir", "compiler/mir/resource", "compiler/mir/ownership", "compiler/mir/reach", "compiler/mir/unwind"},
	"compiler/mir/reach":            {"compiler/mir/ir", "compiler/mir/ops", "compiler/sem", "compiler/syntax"},
	"compiler/mir/unwind":           {"compiler/mir/ir", "compiler/mir/flow", "compiler/sem"},
	"compiler/mir/mobility":         {"lang/diag", "compiler/syntax", "compiler/sem", "compiler/mir", "compiler/mir/flow", "compiler/mir/ir", "compiler/mir/ops"},
	"compiler/mir/build":            {"lang/diag", "compiler/syntax", "compiler/sem", "compiler/hir", "compiler/mir", "compiler/mir/ir", "compiler/mir/mobility", "compiler/mir/ops", "compiler/mir/ownership", "compiler/mir/ownership/borrow", "compiler/mir/reach"},
	"backend/llgen":                 {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "compiler/mir", "compiler/sem/asmtable", "compiler/mir/ops", "compiler/mir/ir", "compiler/mir/layout", "compiler/mir/ownership", "compiler/mir/reach", "compiler/mir/unwind", "backend/llgen/ir", "backend/llgen/runtimegen"},
	"backend/llgen/ir":              {},
	"backend/llgen/runtimegen":      {},
	"base/hashkey":                  {},
	"toolchain/zig":                 {"toolchain/zig/store"},
	"toolchain/zig/store":           {},
	"toolchain/zig/fetch":           {"toolchain/zig/store"},
	"devkit/zigrelease":             {"toolchain/zig/fetch"},
	"toolchain/zig/source":          {"toolchain/zig/store"},
	"interop/cheader":               {"toolchain/zig"},
	"devkit/fixture":                {},
	"devkit/hosttool":               {},
	"devkit/audit":                  {"devkit/fixture", "devkit/hosttool"},
	"devkit/bench":                  {"toolchain/version", "devkit/hosttool"},
	"backend/vm":                    {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "compiler/mir", "backend/vm/host/shell", "toolchain/buildgraph", "backend/vm/host/net", "base/hashkey", "compiler/mir/ops", "compiler/mir/ir"},
	"toolchain/driver":              {"lang/token", "lang/diag", "lang/isr", "compiler/syntax", "compiler/printer", "compiler/sem", "compiler/mir", "backend/llgen", "backend/vm", "toolchain/buildgraph", "toolchain/version", "base/platform", "interop/cheader", "toolchain/contract/schema", "toolchain/zig", "toolchain/zig/store", "toolchain/zig/fetch", "toolchain/policy", "toolchain/config", "toolchain/native/audit", "toolchain/native/object", "compiler/mir/ops", "compiler/mir/ir", "compiler/mir/resource", "compiler/mir/build", "compiler/mir/plan", "compiler/mir/reach", "compiler/mir/verify", "toolchain/artifact", "toolchain/module", "toolchain/native/link"},
	"tooling/lsp":                   {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "toolchain/driver", "base/errors", "toolchain/version", "toolchain/config", "toolchain/module"},
	"base/errors":                   {},
	"base/fsx":                      {},
	"cli/exit":                      {"base/errors"},
	"cli/session":                   {"cli/exit", "toolchain/driver", "base/errors", "toolchain/policy", "toolchain/config", "toolchain/module"},
	"cli/clitest":                   {"cli", "cli/exit", "cli/session"},
	"cli/cmd/parse":                 {"cli/session", "cli/exit", "toolchain/driver", "base/errors"},
	"cli/cmd/fmt":                   {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "base/fsx"},
	"cli/cmd/check":                 {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/policy"},
	"cli/cmd/run":                   {"cli/session", "cli/exit", "toolchain/driver", "base/errors"},
	"cli/cmd/test":                  {"cli/session", "cli/exit", "toolchain/driver", "base/errors"},
	"cli/cmd/build":                 {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/contract/schema", "toolchain/policy"},
	"cli/cmd/lsp":                   {"cli/session", "base/errors", "tooling/lsp"},
	"cli/cmd/doc":                   {"cli/session", "cli/exit", "tooling/doc", "toolchain/driver", "base/errors"},
	"cli/cmd/schema":                {"cli/session", "cli/exit", "tooling/doc", "toolchain/driver", "base/errors"},
	"cli/cmd/doctor":                {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/version", "toolchain/zig", "toolchain/module"},
	"cli/cmd/explain":               {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "compiler/sem", "toolchain/artifact"},
	"cli/cmd/get":                   {"cli/session", "cli/exit", "base/errors", "toolchain/module", "toolchain/module/vcs"},
	"cli/cmd/mod":                   {"cli/session", "cli/exit", "toolchain/driver", "base/errors", "toolchain/version", "toolchain/module"},
	"cli/cmd/zig":                   {"cli/session", "base/errors", "toolchain/version", "compiler/sem", "toolchain/zig", "toolchain/zig/store", "toolchain/zig/fetch", "toolchain/zig/source", "toolchain/module"},
	"toolchain/contract/schema":     {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "toolchain/version", "base/platform"},
	"tooling/doc":                   {"lang/token", "lang/diag", "compiler/syntax", "compiler/sem", "toolchain/driver", "base/errors", "toolchain/version", "toolchain/contract/schema", "base/fsx"},
	"cli":                           {"toolchain/version", "cli/cmd/build", "cli/cmd/check", "cli/cmd/doc", "cli/cmd/doctor", "cli/cmd/fmt", "cli/cmd/get", "cli/cmd/lsp", "cli/cmd/mod", "cli/cmd/parse", "cli/cmd/run", "cli/cmd/schema", "cli/cmd/test", "cli/exit", "base/errors", "cli/cmd/explain", "cli/session", "lang/diag", "cli/cmd/zig"},
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
