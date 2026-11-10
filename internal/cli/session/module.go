package session

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

// StdRoot resolves the std directory: --std, $KIGUMI_STD, ./std, one next to the executable, or the embedded copy.
func (s Session) StdRoot() string {
	if s.Std != "" {
		return s.Std
	}
	if env := os.Getenv("KIGUMI_STD"); env != "" {
		return env
	}
	candidates := []string{"std"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "std"), filepath.Join(filepath.Dir(exe), "..", "std"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "prelude")); err == nil && st.IsDir() {
			return c
		}
	}
	return embeddedStdRoot()
}

// Load reads the module root or entry file target under s.Policy; parse diagnostics go to errOut and end the command with 1.
func (s Session) Load(target string, test bool, errOut io.Writer) (*driver.Module, error) {
	return s.load(target, test, false, false, true, errOut)
}

// LoadWhole is Load for a command that describes the whole module even when the target names one entry file (doc, schema).
func (s Session) LoadWhole(target string, test bool, errOut io.Writer) (*driver.Module, error) {
	return s.load(target, test, false, true, true, errOut)
}

// LoadSettled is Load for a command that only needs the check outcome: an unchanged module answers from the package cache without parsing a source.
func (s Session) LoadSettled(target string, test bool, errOut io.Writer) (*driver.Module, error) {
	return s.load(target, test, true, false, true, errOut)
}

// LoadReport is Load for a command that renders the module's diagnostics
// itself, in the format the caller chose: it prints nothing and parses every
// reachable file so the diagnostics keep their structure. A module that does
// not parse comes back with HasParseErrors set instead of an error.
func (s Session) LoadReport(target string, test bool) (*driver.Module, error) {
	return s.load(target, test, false, false, false, nil)
}

func (s Session) load(target string, test, deferParse, whole, report bool, errOut io.Writer) (*driver.Module, error) {
	root, entry := target, ""
	if strings.HasSuffix(target, ".kg") {
		if _, err := os.Stat(target); err != nil {
			return nil, errors.WrapErrf(err, "load module %s", target)
		}
	} else {
		target = packageEntry(target)
	}
	if strings.HasSuffix(target, ".kg") {
		root = moduleRoot(target)
		rel, err := filepath.Rel(root, target)
		if err != nil {
			return nil, errors.WrapErr(err, "resolve entry file")
		}
		entry = filepath.ToSlash(rel)
	}
	tgt, err := s.Target()
	if err != nil {
		return nil, &exit.ValueError{Err: err}
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: s.StdRoot(), Test: test, Entry: entry, NoLocal: s.NoLocal || LocalOff(), Target: tgt, Policy: s.Policy(), ZigPath: ZigPath(), ZigHome: ZigHome(), Offline: s.Offline, VerifyPasses: VerifyPasses(), Compiler: CompilerOptions(), PackageCache: PackageCache(), DeferParse: deferParse, UnitCheck: deferParse && UnitCheck(), NoSettled: !UnitCheck(), LazyStd: UnitCheck(), AllOwnPackages: whole})
	if err != nil {
		return nil, errors.WrapErrf(err, "load module %s", root)
	}
	if report {
		if m.ReportDiagnostics(errOut) {
			return nil, exit.Code(1)
		}
		for _, v := range m.UnusedReplaces {
			fmt.Fprintf(errOut, "warning[%s]: %s\n", v.ID, v.Subject)
		}
	}
	return m, nil
}

// packageEntry turns a package directory inside a module (`cmd/app`) into main.kg or its only .kg file; anything else
// passes through. Only a manifest makes the enclosing directory a module here: a directory that merely sits under a
// `cmd/` root of an unrelated checkout (Go tooling, say) stays its own root, which is what the same path spelled
// relatively already does.
func packageEntry(dir string) string {
	if module.IsModuleRoot(dir) {
		return dir
	}
	if _, ok := manifestRoot(dir); !ok {
		return dir
	}
	names, _ := filepath.Glob(filepath.Join(dir, "*.kg"))
	for _, n := range names {
		if filepath.Base(n) == "main.kg" {
			return n
		}
	}
	if len(names) == 1 && !module.IsManifest(filepath.Base(names[0])) {
		return names[0]
	}
	return dir
}

// manifestRoot walks up from dir to the nearest directory holding mod.kg.
func manifestRoot(dir string) (string, bool) {
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, module.ManifestName)); err == nil {
			return d, true
		}
		if filepath.Dir(d) == d {
			return "", false
		}
	}
}

// moduleRoot is the nearest ancestor of entry holding mod.kg or cmd/, else entry's own directory (script mode).
func moduleRoot(entry string) string {
	if root, ok := module.FindModuleRoot(filepath.Dir(entry)); ok {
		return root
	}
	return filepath.Dir(entry)
}
