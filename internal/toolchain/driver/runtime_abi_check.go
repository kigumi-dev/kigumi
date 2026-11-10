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
// may have produced: libraries named by the manifest, archive or object paths
// given as flags, and the static archive each -l<name> resolves to under the
// -L directories, searched in the order the linker sees them.
func linkedArtifacts(m *Module, cflags, lflags []string) []string {
	var paths []string
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, l := range m.Links {
		if l.Library != "" && l.Search != "" {
			search := l.Search
			if !filepath.IsAbs(search) {
				if abs, err := m.compiler.Abs(search); err == nil {
					search = abs
				}
			}
			add(filepath.Join(search, "lib"+l.Library+".a"))
		}
	}
	flags := append(append([]string{}, cflags...), lflags...)
	for _, f := range flags {
		if strings.HasSuffix(f, ".a") || strings.HasSuffix(f, ".o") {
			add(f)
		}
	}
	search, libs := linkerSearchFlags(flags)
	for _, name := range libs {
		for _, dir := range search {
			// The first directory holding the archive is the one ld would take.
			if p := filepath.Join(dir, "lib"+name+".a"); fileExists(p) {
				add(p)
				break
			}
		}
	}
	return paths
}

// linkerSearchFlags collects the -L directories and -l names one link searches, in the plain (-L dir), joined (-Ldir) and -Wl, forms
// the driver passes through; a name with no archive under any -L directory is left out by the caller, which is how a system library
// (-lm, -lc) stays unexamined.
func linkerSearchFlags(flags []string) (search, libs []string) {
	value := func(flag, arg string) {
		if flag == "-l" {
			libs = append(libs, arg)
			return
		}
		search = append(search, arg)
	}
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		if f == "-L" || f == "-l" || f == "--library-path" {
			if i+1 < len(flags) {
				value(f, flags[i+1])
				i++
			}
			continue
		}
		parts := []string{f}
		if strings.HasPrefix(f, "-Wl,") {
			parts = strings.Split(f[len("-Wl,"):], ",")
		}
		for j := 0; j < len(parts); j++ {
			p := parts[j]
			switch {
			case p == "-L" || p == "-l" || p == "--library-path":
				if j+1 < len(parts) {
					value(p, parts[j+1])
					j++
				}
			case strings.HasPrefix(p, "--library-path="):
				search = append(search, strings.TrimPrefix(p, "--library-path="))
			case strings.HasPrefix(p, "-L"):
				search = append(search, p[len("-L"):])
			case strings.HasPrefix(p, "-l"):
				libs = append(libs, p[len("-l"):])
			}
		}
	}
	return search, libs
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// checkLinkedRuntimeABI refuses to link a Kigumi-built archive or object
// whose sidecar records a runtime other than the one this build compiles: the two would disagree on layout and op ids. A sidecar
// that exists but does not parse fails closed too: an archive whose runtime cannot be read is not one this build may link.
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
