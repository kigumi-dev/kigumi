package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"kigumi/internal/base/fsx"
)

// PublishWithFingerprint publishes out together with the sidecar that describes it: the sidecar is written first, and only then is the
// output renamed into place, so a reader never meets an artifact whose sidecar is missing or still describes the build it replaced. A
// crash between the two writes leaves a sidecar newer than the artifact, which the runtime-ABI guard reports as a mismatch: it rejects
// one good archive rather than link an archive it cannot describe.
func PublishWithFingerprint(out string, fp Fingerprint, build func(staged string) error) (bool, error) {
	side := FingerprintSidecarPath(out)
	b, err := json.MarshalIndent(fp, "", "  ")
	if err != nil {
		return false, err
	}
	prev, prevErr := os.ReadFile(side)
	hadPrev := prevErr == nil
	if err := fsx.WriteFileAtomic(side, b, 0o644); err != nil {
		return false, err
	}
	ok, err := Publish(out, build)
	if ok && err == nil {
		return true, nil
	}
	// The build failed: keep the sidecar describing the artifact still in place.
	if hadPrev {
		_ = fsx.WriteFileAtomic(side, prev, 0o644)
	} else {
		_ = os.Remove(side)
	}
	return ok, err
}

// Publish stages build's output and only renames it over out once it's a non-empty regular file, so a failed in-place linker/archiver can't destroy the last good out.
func Publish(out string, build func(staged string) error) (bool, error) {
	dir := filepath.Dir(out)
	f, err := os.CreateTemp(dir, "."+filepath.Base(out)+".build-")
	if err != nil {
		return false, err
	}
	staged := f.Name()
	f.Close()
	// ar refuses an existing empty file as a fresh archive; every build
	// (cc/ld included) is fine with the name just not existing yet.
	os.Remove(staged)
	defer os.Remove(staged)
	if err := build(staged); err != nil {
		return false, err
	}
	fi, err := os.Lstat(staged)
	if err != nil {
		return false, fmt.Errorf("build reported success but wrote nothing at %s: %w", out, err)
	}
	if !fi.Mode().IsRegular() || fi.Size() == 0 {
		return false, fmt.Errorf("build reported success but left no usable output at %s", out)
	}
	if err := os.Rename(staged, out); err != nil {
		return false, err
	}
	return true, nil
}
