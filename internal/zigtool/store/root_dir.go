// Package store manages kigumi's own zig installations: one directory per
// version, each holding the unpacked release plus a manifest.json.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Root is the managed store's root directory: override when non-empty
// (internal/cli resolves env vars into this; this package reads none itself), otherwise ~/.local/share/kigumi/zig.
func Root(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve the zig store directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "kigumi", "zig"), nil
}

// VersionDir is where version's unpacked release and manifest.json live
// under root.
func VersionDir(root, version string) string {
	return filepath.Join(root, version)
}

// validateVersion rejects a version unsafe as a store directory name;
// mod.kg's `zig:` field and CLI args reach here unsanitized, so every entry point below calls this first.
func validateVersion(version string) error {
	if version == "" || version == "." || version == ".." ||
		strings.ContainsAny(version, "/\\") {
		return fmt.Errorf("invalid zig version %q", version)
	}
	return nil
}

// BinaryName is the zig executable's name inside a version directory.
func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "zig.exe"
	}
	return "zig"
}

// BinaryPath is the zig executable inside versionDir.
func BinaryPath(versionDir string) string {
	return filepath.Join(versionDir, BinaryName())
}
