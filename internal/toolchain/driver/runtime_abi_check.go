package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/artifact"
)

// linkedArtifacts lists the files this link pulls in that a Kigumi build
// may have produced: libraries named by the manifest and archive or object paths given as flags.
func linkedArtifacts(m *Module, cflags, lflags []string) []string {
	var paths []string
	for _, l := range m.Links {
		if l.Library != "" && l.Search != "" {
			search := l.Search
			if !filepath.IsAbs(search) {
				if abs, err := m.compiler.Abs(search); err == nil {
					search = abs
				}
			}
			paths = append(paths, filepath.Join(search, "lib"+l.Library+".a"))
		}
	}
	for _, f := range append(append([]string{}, cflags...), lflags...) {
		if strings.HasSuffix(f, ".a") || strings.HasSuffix(f, ".o") {
			paths = append(paths, f)
		}
	}
	return paths
}

// checkLinkedRuntimeABI refuses to link a Kigumi-built archive or object
// whose sidecar records a runtime other than the one this build compiles: the two would disagree on layout and op ids.
func checkLinkedRuntimeABI(m *Module, cflags, lflags []string, want string) error {
	for _, path := range linkedArtifacts(m, cflags, lflags) {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		fp, ok, err := artifact.ReadFingerprintSidecar(path)
		if err != nil {
			msg, help := sem.FillCode(sem.RuntimeAbiUnreadable, path, err)
			return fmt.Errorf("%s: %s: %s (%s)", sem.RuntimeAbiUnreadable.Num, sem.RuntimeAbiUnreadable.Name, msg, help)
		}
		if !ok || fp.RuntimeAbi == want {
			continue
		}
		msg, help := sem.FillCode(sem.RuntimeAbiMismatch, path, artifact.OrNone(fp.RuntimeAbi), want)
		return fmt.Errorf("%s: %s: %s (%s)", sem.RuntimeAbiMismatch.Num, sem.RuntimeAbiMismatch.Name, msg, help)
	}
	return nil
}
