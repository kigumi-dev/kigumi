package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ManifestFile is manifest.json's name inside a version directory.
const ManifestFile = "manifest.json"

// Manifest records one installed version: what was fetched, for what host,
// and how, so `kigumi zig list` and `which` can explain it without re-deriving it.
type Manifest struct {
	Version     string    `json:"version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	TarballURL  string    `json:"tarballUrl,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	InstalledAt time.Time `json:"installedAt"`
	// Obtained is "release" (an official prebuilt tarball) or "source" (built locally).
	Obtained string `json:"obtained"`
	// Commit and Dirty are set only when Obtained is "source".
	Commit string `json:"commit,omitempty"`
	Dirty  bool   `json:"dirty,omitempty"`
}

func ReadManifest(versionDir string) (Manifest, error) {
	var m Manifest
	data, err := os.ReadFile(filepath.Join(versionDir, ManifestFile))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parse %s: %w", filepath.Join(versionDir, ManifestFile), err)
	}
	return m, nil
}

func WriteManifest(versionDir string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(versionDir, ManifestFile), data, 0o644)
}
