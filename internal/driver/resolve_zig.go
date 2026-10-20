package driver

import (
	"fmt"
	"os"
	"path/filepath"

	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/token"
	"kigumi/internal/zigtool"
	"kigumi/internal/zigtool/fetch"
	"kigumi/internal/zigtool/store"
)

// ResolveZig resolves zig per zigtool.ForModule's order (store, explicit,
// PATH); an unsatisfied pin fetches it on demand unless m.offline (E1563).
func (m *Module) ResolveZig() (string, error) {
	storeRoot, _ := m.compiler.ZigStoreRoot(m.zigHome)
	r := zigtool.ForModule(m.compiler.ToolLookup(), storeRoot, m.explicitZig, m.Manifest.Zig)
	switch {
	case r.Want == "":
		return r.Path, r.Err
	case r.Err == nil && !r.Mismatch:
		return r.Path, nil
	case m.offline:
		return "", m.zigPinError(sem.ZigOfflineUnresolved, r.Want, r.Want)
	}
	path, err := m.fetchZigOnDemand(storeRoot, r.Want)
	if err != nil {
		return "", m.zigPinError(sem.ZigFetchUnavailable, r.Want, err.Error(), r.Want)
	}
	return path, nil
}

// fetchZigOnDemand installs want into storeRoot via internal/zigtool/fetch,
// noting it on m.zigNotice (os.Stderr by default) first.
func (m *Module) fetchZigOnDemand(storeRoot, want string) (string, error) {
	notice := m.zigNotice
	if notice == nil {
		notice = os.Stderr
	}
	fmt.Fprintf(notice, "kigumi: zig %s is pinned but not installed; fetching it now (run `kigumi zig install` to do this ahead of time)\n", want)
	if _, err := fetch.InstallRelease(fetch.Options{IndexURL: m.zigIndexURL, StoreRoot: storeRoot, Version: want, Stderr: notice, PublicKey: m.zigPublicKey}); err != nil {
		return "", err
	}
	path, ok := store.Installed(storeRoot, want)
	if !ok {
		return "", fmt.Errorf("zig %s was installed but is not where the store expects it", want)
	}
	return path, nil
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
	return m.compiler.CCompiler(), nil
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
