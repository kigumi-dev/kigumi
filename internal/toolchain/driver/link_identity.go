package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/policy"
)

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

// ResolveLinkIdentity locates and digests the actual library file a
// Link{library: ...} names; ok is false when nothing on t's search list resolves it.
func ResolveLinkIdentity(o buildconfig.CompilerOptions, l Link, t buildconfig.Target) (LinkIdentity, bool) {
	id := LinkIdentity{Library: l.Library}
	if l.Library == "" || !IsHostTarget(o, t) {
		return id, false
	}
	hostOS := o.HostOrAmbient().OS
	dirs := defaultLibraryDirs(hostOS)
	if l.Search != "" {
		dirs = append([]string{l.Search}, dirs...)
	}
	for _, dir := range dirs {
		for _, name := range candidateLibraryNames(hostOS, l.Library) {
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

// linkIdentityUnknownViolation is distinct from release-native-link-unresolved:
// that one is a missing Search directory, this one a Search directory present but not holding the named library.
func linkIdentityUnknownViolation(module, library string) policy.Violation {
	return policy.Violation{
		ID:      "release-native-link-identity-unknown",
		Subject: "Link { library: \"" + library + "\" } (" + module + ") does not resolve to a library file this host's search list can find",
		Detail:  []policy.KV{policy.Detail("library", library)},
		Policy:  "rejected because release requires the actual resolved library identity to be recorded, not only the declared name",
		Help:    "add a Link search path that holds lib" + library + ", install the library where this host's linker would find it, or keep this as a development build",
	}
}

// LinkIdentityViolations is EvaluateRelease's extra argument for every
// Link in m that ResolveLinkIdentity could not find against t.
func LinkIdentityViolations(m *Module, t buildconfig.Target) []policy.Violation {
	var vs []policy.Violation
	for _, l := range m.Links {
		if l.Library == "" {
			continue
		}
		if _, ok := ResolveLinkIdentity(m.compiler, l, t); !ok {
			vs = append(vs, linkIdentityUnknownViolation(m.Name, l.Library))
		}
	}
	return vs
}
