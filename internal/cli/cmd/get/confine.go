package get

import (
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/toolchain/driver"
)

func confinedSubdir(tmp, subdir string, r driver.Require) (string, error) {
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		return "", errors.WrapErr(err, "resolve the checkout")
	}
	src, err := filepath.EvalSymlinks(filepath.Join(realTmp, filepath.FromSlash(subdir)))
	if err != nil {
		return "", errors.NewErrf("%s at %s has no %s in %q", r.Name, r.Version, driver.ManifestName, subdir)
	}
	rel, err := filepath.Rel(realTmp, src)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.NewErrf("%s %s: subdirectory %q resolves outside the fetched repository (a symlink or `..`); refusing to use it", r.Name, r.Version, subdir)
	}
	return src, nil
}
