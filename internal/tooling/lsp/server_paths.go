package lsp

import (
	"os"
	"path/filepath"
	"strings"

	"go.lsp.dev/uri"

	"kigumi/internal/toolchain/module"
)

// findStd locates the std stubs when the CLI passed none: <root>/std, or
// next to the executable.
func findStd(root string) string {
	candidates := []string{filepath.Join(root, "std")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "std"), filepath.Join(filepath.Dir(exe), "..", "std"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "prelude")); err == nil && st.IsDir() {
			return c
		}
	}
	return ""
}

// uriToPath delegates to uri.URI.FsPath, which (unlike a hand-rolled
// url.Parse + FromSlash) handles authority, Windows drives and UNC shares.
func uriToPath(u uri.URI) string {
	if !u.IsFile() {
		return string(u)
	}
	return u.FsPath()
}

// allModuleRoots dedups module roots found under overlapping or nested
// workspace folders (VS Code allows adding both a folder and its subdir).
func (s *Server) allModuleRoots() []string {
	seen := map[string]bool{}
	var roots []string
	for _, workspace := range s.roots {
		for _, root := range module.Roots(s.compiler, workspace, s.stdRoot) {
			root = filepath.Clean(root)
			if seen[root] {
				continue
			}
			seen[root] = true
			roots = append(roots, root)
		}
	}
	return roots
}

// moduleRoot picks the nearest ancestor module root inside a workspace
// folder, or outside every folder the file's own directory (script mode).
func (s *Server) moduleRoot(path string) string {
	for _, root := range s.roots {
		if root == "" || !strings.HasPrefix(path, root+string(filepath.Separator)) {
			continue
		}
		for dir := filepath.Dir(path); dir != root; dir = filepath.Dir(dir) {
			if module.IsModuleRoot(dir) {
				return dir
			}
		}
		return root
	}
	if root, ok := module.FindModuleRoot(filepath.Dir(path)); ok {
		return root
	}
	return filepath.Dir(path)
}
