package artifact

import (
	"fmt"
	"os"

	"kigumi/internal/base/fsx"
)

// CheckOutputSlot refuses a symlink, directory or special file already at path; writing through a symlink there could land outside the output directory.
func CheckOutputSlot(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("output %s already exists as a symlink", path)
	case fi.IsDir():
		return fmt.Errorf("output %s already exists as a directory", path)
	case !fi.Mode().IsRegular():
		return fmt.Errorf("output %s already exists as a special file", path)
	}
	return nil
}

// WriteOutput atomically writes content at path once CheckOutputSlot passes; unlike fsx.WriteFileAtomic it enforces perm
// over an existing file, so a stale or tampered output directory cannot leak its old mode into generated content.
func WriteOutput(path string, content []byte, perm os.FileMode) error {
	if err := CheckOutputSlot(path); err != nil {
		return err
	}
	if err := fsx.WriteFileAtomic(path, content, perm); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}
