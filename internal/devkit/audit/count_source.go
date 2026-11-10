package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PackageCounts is one Go package directory's test inventory. SkipSites counts
// .Skip(/.Skipf( call sites in the test sources, not runtime skip events: a
// shared helper counts once per call site and an environment-dependent skip is
// invisible here. Runtime skips belong in a `go test -json` baseline instead.
type PackageCounts struct {
	Dir        string `json:"dir"`
	Tests      int    `json:"tests"`
	Benchmarks int    `json:"benchmarks"`
	Fuzz       int    `json:"fuzz"`
	SkipSites  int    `json:"skipSites"`
	HasTests   bool   `json:"hasTests"`
}

// testKind classifies a top-level function name the way `go test` does:
// Test/Benchmark/Fuzz followed by nothing lowercase.
func testKind(name string) string {
	for prefix, kind := range map[string]string{"Test": "test", "Benchmark": "benchmark", "Fuzz": "fuzz"} {
		rest, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		if rest == "" {
			return kind
		}
		if r, _ := utf8.DecodeRuneInString(rest); !unicode.IsLower(r) {
			return kind
		}
	}
	return ""
}

// CountGoTests tallies Test/Benchmark/Fuzz functions and .Skip( call sites
// (SkipSites, not runtime skip events) per package dir under repoRoot; skipDirs
// (by base name) are pruned.
func CountGoTests(repoRoot string, skipDirs ...string) ([]PackageCounts, error) {
	byDir := map[string]*PackageCounts{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			for _, skip := range skipDirs {
				if d.Name() == skip {
					return fs.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(p) != ".go" {
			return nil
		}
		dir, _ := filepath.Rel(repoRoot, filepath.Dir(p))
		dir = filepath.ToSlash(dir)
		pc := byDir[dir]
		if pc == nil {
			pc = &PackageCounts{Dir: dir}
			byDir[dir] = pc
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		pc.HasTests = true
		file, perr := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			switch testKind(fn.Name.Name) {
			case "test":
				pc.Tests++
			case "benchmark":
				pc.Benchmarks++
			case "fuzz":
				pc.Fuzz++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]PackageCounts, 0, len(byDir))
	for _, pc := range byDir {
		out = append(out, *pc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	for i := range out {
		n, err := countSkipSites(repoRoot, out[i].Dir)
		if err != nil {
			return nil, err
		}
		out[i].SkipSites = n
	}
	return out, nil
}

func countSkipSites(repoRoot, dir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(repoRoot, dir, "*_test.go"))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range files {
		b, err := readFile(f)
		if err != nil {
			return 0, err
		}
		n += strings.Count(b, ".Skip(") + strings.Count(b, ".Skipf(")
	}
	return n, nil
}
