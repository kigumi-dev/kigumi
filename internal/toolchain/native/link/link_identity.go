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

// defaultLibraryDirs is searched after a Link's own Search path.
func defaultLibraryDirs(hostOS string) []string {
	switch hostOS {
	case "darwin":
		return []string{"/usr/lib", "/usr/local/lib", "/opt/homebrew/lib"}
	case "windows":
		return nil
	default:
		return []string{"/usr/lib", "/usr/lib/x86_64-linux-gnu", "/usr/lib/aarch64-linux-gnu", "/usr/lib64", "/lib", "/lib/x86_64-linux-gnu", "/usr/local/lib"}
	}
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
	hostOS := o.HostOrAmbient().OS
	dirs := defaultLibraryDirs(hostOS)
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
