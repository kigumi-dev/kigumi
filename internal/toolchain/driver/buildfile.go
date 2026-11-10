package driver

import (
	"fmt"

	"kigumi/internal/compiler/syntax"
)

// BuildFileName is the build program at a module root: ordinary Kigumi
// source with a single `fn configure(b: build.Builder) -> Void!`.
const BuildFileName = "build.kg"

// ReadBuildFile parses root/build.kg; ok is false when there is none.
func ReadBuildFile(root string) (*syntax.Tree, bool, error) {
	t, ok, err := parseBuildFile(root, nil)
	if err != nil || !ok {
		return nil, ok, err
	}
	if t.HasErrors() {
		return t, true, fmt.Errorf("%s:\n%s", BuildFileName, diagRender(t))
	}
	return t, true, nil
}
