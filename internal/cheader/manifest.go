package cheader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// SourceFile is one header that contributed to a cache entry: the entry
// header itself, or one reached through its #include closure.
type SourceFile struct {
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
	Entry  bool   `json:"entry,omitempty"`
}

// Manifest is a cache entry's full identity, published next to gen.kg:
// what was read, the tool and argv, and what was skipped.
type Manifest struct {
	FormatVersion     int          `json:"formatVersion"`
	ZigDigest         string       `json:"zigDigest"`
	TranslatorVersion string       `json:"translatorVersion"`
	Target            string       `json:"target,omitempty"`
	ABI               ABI          `json:"abi"`
	Argv              []string     `json:"argv"`
	Sources           []SourceFile `json:"sources"`
	Skipped           []string     `json:"skipped,omitempty"`
}

func readManifest(dir string) (Manifest, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Manifest{}, false
	}
	var mf Manifest
	if json.Unmarshal(b, &mf) != nil {
		return Manifest{}, false
	}
	return mf, true
}

// publishEntry renames a fresh gen.kg+manifest.json pair into dir in one
// step; a losing concurrent writer's rename failure is not an error.
func publishEntry(cacheDir, dir, source string, mf Manifest) error {
	tmp, err := os.MkdirTemp(cacheDir, ".tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, "gen.kg"), []byte(source), 0o644); err != nil {
		return err
	}
	b, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, dir); err != nil {
		if _, statErr := os.Stat(filepath.Join(dir, "manifest.json")); statErr == nil {
			return nil
		}
		return err
	}
	return nil
}

// hashFile sha256-sums a file's content, for closure membership in the
// cache key and in the published manifest.
func hashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
