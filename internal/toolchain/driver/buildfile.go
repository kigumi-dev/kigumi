package driver

import (
	"fmt"
	"os"
	"path/filepath"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// BuildFileName is the build program at a module root: ordinary Kigumi
// source with a single `fn configure(b: build.Builder) -> Void!`.
const BuildFileName = "build.kg"

// ReadBuildFile parses root/build.kg; ok is false when there is none.
func ReadBuildFile(root string) (*syntax.Tree, bool, error) {
	src, err := os.ReadFile(filepath.Join(root, BuildFileName))
	if err != nil {
		return nil, false, nil
	}
	t := syntax.Parse(token.NewFile(BuildFileName, src))
	if t.HasErrors() {
		return t, true, fmt.Errorf("%s:\n%s", BuildFileName, diagRender(t))
	}
	return t, true, nil
}
