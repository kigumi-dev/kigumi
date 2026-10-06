package driver

import (
	"fmt"
	"os"
	"path/filepath"

	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/token"
	"kigumi/internal/zigtool"
)

// ResolveZig tries explicitZig then PATH, verified against the manifest's
// pin; "" with a nil error means no zig at all, so the caller may fall back to clang.
func (m *Module) ResolveZig() (string, error) {
	c := zigtool.CheckZig(m.explicitZig, m.Manifest.Zig)
	switch {
	case c.Err == nil && !c.Mismatch:
		return c.Path, nil
	case c.Want == "":
		return "", c.Err
	case c.Mismatch:
		return "", m.zigPinError(sem.ZigVersionMismatch, c.Path, c.Version, c.Want)
	case c.Path == "":
		return "", m.zigPinError(sem.ZigMissing, c.Want, c.Err.Error())
	default:
		return "", m.zigPinError(sem.ZigVersionUnknown, c.Path, c.Err.Error(), c.Want)
	}
}

// zigPinError points the error at the manifest's `zig`/`Zig` field; args
// split across c's message and help holes the same way a sem.Result diagnostic's do.
func (m *Module) zigPinError(c sem.Code, args ...any) error {
	msg, help := sem.FillCode(c, args...)
	name, path := ManifestName, filepath.Join(m.Root, ManifestName)
	if m.ScriptFile != "" {
		name, path = filepath.Base(m.ScriptFile), filepath.Join(m.Root, filepath.FromSlash(m.ScriptFile))
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %s: %s", path, c.Name, msg)
	}
	f := token.NewFile(name, src)
	d := diag.Errorf(diag.At(f, m.Manifest.ZigSpan), msg).WithHelp(help)
	d.Code = c.Num
	return fmt.Errorf("%s: %s", path, diag.RenderAll(f, []diag.Diagnostic{d}))
}

// CCompilerFor is buildOnce's C compiler: m's pinned zig when its manifest
// declares one, otherwise the plain zig-then-clang search (CCompiler).
func CCompilerFor(m *Module) ([]string, error) {
	path, err := m.ResolveZig()
	if err != nil {
		return nil, err
	}
	if path != "" {
		return []string{path, "cc"}, nil
	}
	return CCompiler(), nil
}

// ZigVersionOf is "" unless cc is a zig invocation, for ArtifactFingerprint
// and the cheader cache key's zig axis.
func ZigVersionOf(cc []string) string {
	if len(cc) == 0 || filepath.Base(cc[0]) != "zig" {
		return ""
	}
	v, err := zigtool.Version(cc[0])
	if err != nil {
		return ""
	}
	return v
}
