package printer_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/compiler/printer"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// TestStdIsCanonical pins std to the formatter's output. A formatter change
// that drifts from what the standard library holds would otherwise reach
// downstream modules unnoticed, because no other test formats std.
func TestStdIsCanonical(t *testing.T) {
	root := "../../../std"
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".kg") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no std sources")
	}
	for _, path := range files {
		t.Run(strings.TrimPrefix(path, root+"/"), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tree := syntax.Parse(token.NewFile(path, src))
			for _, d := range tree.Diags {
				if d.Severity == diag.Error {
					t.Fatalf("parse: %v", d)
				}
			}
			if got := printer.Print(tree); got != string(src) {
				t.Errorf("not in canonical form; run `kigumi fmt -w %s`\n--- canonical ---\n%s", path, got)
			}
		})
	}
}
