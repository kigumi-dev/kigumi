package fetch

import (
	"fmt"
	"os/exec"
)

// extractTarball strips the "zig-<arch>-<os>-<version>/" wrapper every
// official tarball uses; shells out to tar to avoid an archive/xz dependency.
func extractTarball(tarPath, dir string) error {
	cmd := exec.Command("tar", "-xf", tarPath, "-C", dir, "--strip-components=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract %s: %w: %s", tarPath, err, out)
	}
	return nil
}
