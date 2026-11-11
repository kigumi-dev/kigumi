package link

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	buildconfig "kigumi/internal/toolchain/config"
)

// Library is the part of a module's Link record that names a library file: Name as in -lName, Search tried first.
type Library struct {
	Name, Search string
}

// LinkIdentity is what a Link record actually resolved to on this host,
// kept separate from the descriptor the module tree hash already covers.
type LinkIdentity struct {
	Library      string
	ResolvedPath string
	// Digest is a SHA-256 of ResolvedPath; empty when unresolved.
	Digest string
}

func candidateLibraryNames(hostOS, name string) []string {
	switch hostOS {
	case "darwin":
		return []string{"lib" + name + ".dylib", "lib" + name + ".a"}
	case "windows":
		return []string{name + ".lib", "lib" + name + ".a"}
	default:
		return []string{"lib" + name + ".so", "lib" + name + ".a"}
	}
}

// debianMultiarch names the directory the Debian family puts an
// architecture's libraries in; a missing entry means the distribution has
// no such directory, not that the architecture is unsupported.
var debianMultiarch = map[string]string{
	"amd64":    "x86_64-linux-gnu",
	"arm64":    "aarch64-linux-gnu",
	"riscv64":  "riscv64-linux-gnu",
	"386":      "i386-linux-gnu",
	"arm":      "arm-linux-gnueabihf",
	"ppc64":    "powerpc64-linux-gnu",
	"ppc64le":  "powerpc64le-linux-gnu",
	"s390x":    "s390x-linux-gnu",
	"loong64":  "loongarch64-linux-gnu",
	"mips":     "mips-linux-gnu",
	"mipsle":   "mipsel-linux-gnu",
	"mips64":   "mips64-linux-gnuabi64",
	"mips64le": "mips64el-linux-gnuabi64",
}

// defaultLibraryDirs is searched after a Link's own Search path; the
// multiarch directory follows the host's architecture from the target
// layout rather than a hardcoded pair of x86 names.
func defaultLibraryDirs(hostOS, arch string) []string {
	switch hostOS {
	case "darwin":
		return []string{"/usr/lib", "/usr/local/lib", "/opt/homebrew/lib"}
	case "windows":
		return nil
	}
	multi := debianMultiarch[arch]
	dirs := []string{"/usr/lib"}
	if multi != "" {
		dirs = append(dirs, "/usr/lib/"+multi)
	}
	dirs = append(dirs, "/usr/lib64", "/lib")
	if multi != "" {
		dirs = append(dirs, "/lib/"+multi)
	}
	return append(dirs, "/usr/local/lib")
}

// IsHostTarget reports whether t names no cross-compilation: ResolveLinkIdentity's
// search list is the running host's own, which does not apply to a different target.
func IsHostTarget(o buildconfig.CompilerOptions, t buildconfig.Target) bool {
	h := o.HostOrAmbient()
	return t.Triple == "" && (t.OS == "" || t.OS == h.OS) && (t.Arch == "" || t.Arch == h.Arch)
}

// ResolveLinkIdentity locates and digests the actual library file lib
// names; ok is false when nothing on t's search list resolves it.
func ResolveLinkIdentity(o buildconfig.CompilerOptions, lib Library, t buildconfig.Target) (LinkIdentity, bool) {
	id := LinkIdentity{Library: lib.Name}
	if lib.Name == "" || !IsHostTarget(o, t) {
		return id, false
	}
	host := o.HostOrAmbient()
	hostOS := host.OS
	dirs := defaultLibraryDirs(hostOS, host.Arch)
	if lib.Search != "" {
		dirs = append([]string{lib.Search}, dirs...)
	}
	for _, dir := range dirs {
		for _, name := range candidateLibraryNames(hostOS, lib.Name) {
			p := filepath.Join(dir, name)
			digest, err := digestFile(p)
			if err != nil {
				continue
			}
			id.ResolvedPath, id.Digest = p, digest
			return id, true
		}
	}
	return id, false
}

func digestFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
