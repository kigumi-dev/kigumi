package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

var residueSymbols = map[string]bool{
	"exec.LookPath": true, "filepath.Abs": true, "os.UserHomeDir": true, "os.Getwd": true,
	"runtime.GOOS": true, "runtime.GOARCH": true,
}

// residueAllowed freezes the ambient reads still outside CompilerOptions; a new one must go through the options instead.
var residueAllowed = map[string]bool{
	"driver/compiler_options.go os.Getwd": true, "driver/compiler_options.go runtime.GOOS": true,
	"driver/compiler_options.go runtime.GOARCH": true,
	"driver/compiler_tools.go exec.LookPath":    true, "driver/compiler_tools.go os.UserHomeDir": true,
	"zigtool/fetch/host_triple.go runtime.GOOS": true, "zigtool/fetch/host_triple.go runtime.GOARCH": true,
	"zigtool/fetch/install_release.go runtime.GOOS": true, "zigtool/fetch/install_release.go runtime.GOARCH": true,
	"zigtool/store/root_dir.go runtime.GOOS": true,
	"zigtool/source/build.go runtime.GOOS":   true, "zigtool/source/build.go runtime.GOARCH": true,
	"platform/platform.go runtime.GOOS": true, "platform/platform.go runtime.GOARCH": true,
	"target/target.go runtime.GOOS": true, "target/target.go runtime.GOARCH": true,
}

func TestAmbientReadsOutsideOptionsAreFrozen(t *testing.T) {
	seen := map[string]bool{}
	for _, dir := range []string{"driver", "zigtool", "platform", "target"} {
		root := filepath.Join("..", dir)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			f, perr := parser.ParseFile(token.NewFileSet(), p, nil, 0)
			if perr != nil {
				return perr
			}
			rel, _ := filepath.Rel("..", p)
			ast.Inspect(f, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if id, ok := sel.X.(*ast.Ident); ok && residueSymbols[id.Name+"."+sel.Sel.Name] {
					key := filepath.ToSlash(rel) + " " + id.Name + "." + sel.Sel.Name
					seen[key] = true
					if !residueAllowed[key] {
						t.Errorf("%s は process の状態を直接読む。CompilerOptions に載せて渡せ", key)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for key := range residueAllowed {
		if !seen[key] {
			t.Errorf("%s は許可表にあるが読まれていない。表から外せ", key)
		}
	}
}
